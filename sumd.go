package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/decred/politeia/politeiad/api/v1"
	"github.com/decred/politeia/politeiad/api/v1/identity"
	"github.com/decred/politeia/util"
)

type Args struct {
	// the release directory
	ReleaseDir string `long:"reldir" description:"the release directory" required:"true"`
	// the base url of the service
	BaseUrl string `long:"baseurl" description:"the base url of the service" required:"true"`
	// the service port
	Port string `short:"p" long:"port" description:"the listening port" required:"true"`
	// pi's endpoint
	Pi string `long:"pi" description:"pi's endpoint" required:"true"`
}

// CacheRelease represents a cached entry that describes a file
type CachedRelease struct {
	// the product name
	Product string `json:"product"`
	// the version
	Version string `json:"version"`
	// the file
	File string `json:"file"`
	// the record expiry
	Expiry *time.Time
}

// ChecksumMetadata represents metadata entry for a release
// record
type ChecksumMetadata struct {
	// the hash of the release
	Checksum string `json:"checksum"`
	// the name of the software
	Product string `json:"product"`
	// the version number of the release
	Version string `json:"version"`
	// the filename of the release
	File string `json:"file"`
}

// Sumd repsents the checksum service
type Sumd struct {
	// the service args
	Args *Args
	// the release cache
	Cache *map[string]CachedRelease
	// the cache update ticker
	Ticker *time.Ticker
	// the server's identity
	Fi *identity.FullIdentity
	// pi's public identity
	Pipi *identity.PublicIdentity
	// the http client
	HttpClient *http.Client
}

// Constructor
func NewSumd(args *Args) (*Sumd, error) {
	sumd = &Sumd{
		Args:  args,
		Cache: &map[string]CachedRelease{},
		HttpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}

	var err error
	sumd.Fi, err = identity.LoadFullIdentity("identity.json")
	if err != nil {
		return nil, err
	}
	log.Println(">>> identity loaded")
	sumd.Pipi, err = util.RemoteIdentity(true, sumd.Args.Pi, "")
	if err != nil {
		return nil, err
	}
	log.Println(">>> pi public identity fetched")

	// update the release cache every 2 minutes
	sumd.Ticker = time.NewTicker(time.Minute * 2)
	go func() {
		now := time.Now()
		for range sumd.Ticker.C {
			for k, v := range *sumd.Cache {
				if now.Before(*v.Expiry) {
					delete(*sumd.Cache, k)
				}
			}
		}
	}()

	return sumd, nil
}

// checksum generates the hex encoded checksum of a file
func (sumd *Sumd) checksum(file *os.File) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to copy file: %s", err.Error())
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// generateKey generates a hex encoded unique key
func (sumd *Sumd) generateKey() string {
	data, err := Random(6)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(data)
}

// getReleaseFile fetches the a release file
func (sumd *Sumd) getReleaseFile(version string, name, releasefile string, releaseDir string) (*os.File, error) {
	releasePath := fmt.Sprintf("%s/%s/%s/%s", releaseDir, name, version, releasefile)
	file, err := os.Open(releasePath)
	if err != nil {
		fmt.Printf("failed to release file: %s", err)
		return nil, errors.New("release file not found")
	}
	return file, nil
}

// generateReleaseKey caches a verified release for future downloads
func (sumd *Sumd) cacheRelease(product string, version string, file string) (string, error) {
	now := time.Now()
	cachedRelease := &CachedRelease{
		Product: product,
		Version: version,
		File:    file,
		Expiry:  GetFutureTime(&now, 1, 0, 0, 0),
	}

	key := sumd.generateKey()
	(*sumd.Cache)[key] = *cachedRelease
	return key, nil
}

// formUrl generates the download url for a cached release
func (sumd *Sumd) formUrl(key string, file string) string {
	return fmt.Sprintf("%s%s/download/%s/%s", sumd.Args.BaseUrl, sumd.Args.Port, key, file)
}

// The metadata payload is structured as follows:
// {
//   "checksum":"hash", // the hash of the release
// 	 "product": "name", // the name of the software
//   "version": "version number", // the version number of the release
//   "file": "filename", // the filename of the release
// }
//
// The payload returned is structured as follows:
// on successful verification:
// {
//   "releasechecksum": "hash",
//   "distributionchecksum": "hash",
//   "verified": true,
//   "download": "url",
// }
//
// on verification failure:
// {
//   "releasechecksum": "hash",
//   "distributionchecksum": "hash",
//   "verified": false,
//   "error": "details",
// }

// ChecksumVerify verifies the distribution checksum against the actual
// release checksum, it returns a payload with a download link if the
// checksums match.
func (sumd *Sumd) verify(token string, product string, version string, filename string) (map[string]interface{}, error) {
	getVettedPayload := v1.GetVetted{
		Challenge: hex.EncodeToString(sumd.Fi.Public.Key[:]),
		Token:     token,
	}
	payloadBytes, err := json.Marshal(getVettedPayload)
	if err != nil {
		return nil, err
	}

	// fetch release record
	req, err := http.NewRequest("POST", sumd.Args.Pi+v1.GetVettedRoute,
		bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	resp, err := sumd.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body := util.ConvertBodyToByteArray(resp.Body, false)
	reply := &v1.GetVettedReply{}
	err = json.Unmarshal(body, reply)
	if err != nil {
		return nil, err
	}
	sigBytes := []byte(reply.Record.CensorshipRecord.Signature)
	var sig [identity.SignatureSize]byte
	copy(sig[:], sigBytes)

	// verify pi response & client challenge
	if sumd.Pipi.VerifyMessage(body, sig) {
		return nil, errors.New("message verification failed")
	}
	err = util.VerifyChallenge(sumd.Pipi, sumd.Fi.Public.Key[:], reply.Response)
	if err != nil {
		return nil, err
	}

	// fetch the requested release checksum record
	requestedMetadata := &ChecksumMetadata{}
	metadataSet := &reply.Record.Metadata
	for _, metadata := range *metadataSet {
		err := json.Unmarshal([]byte(metadata.Payload), requestedMetadata)
		if err != nil {
			return nil, err
		}

		if product == requestedMetadata.Product && version == requestedMetadata.Version && filename == requestedMetadata.File {
			break
		}
	}

	if requestedMetadata.Checksum == "" {
		return nil, fmt.Errorf("no metadata found for record with token %s", token)
	}

	file, err := sumd.getReleaseFile(requestedMetadata.Version, requestedMetadata.Product, requestedMetadata.File, sumd.Args.ReleaseDir)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	releaseSum, err := sumd.checksum(file)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"releasechecksum":      releaseSum,
		"distributionchecksum": requestedMetadata.Checksum,
	}

	if requestedMetadata.Checksum == releaseSum {
		payload["verified"] = true
		key, err := sumd.cacheRelease(requestedMetadata.Product, requestedMetadata.Version, requestedMetadata.File)
		if err != nil {
			return nil, fmt.Errorf("failed to cache release file: %s", err)
		}
		url := sumd.formUrl(key, requestedMetadata.File)
		payload["download"] = url
	} else {
		payload["verified"] = false
		payload["error"] = map[string]string{
			"msg": "data integrity check failed for the requested file, the download has been aborted for your safety.",
		}
	}

	return payload, nil
}
