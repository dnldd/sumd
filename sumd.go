package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

type Args struct {
	// The release directory
	ReleaseDir string `long:"reldir" description:"the release directory" required:"true"`
	// The base url of the service
	BaseUrl string `long:"baseurl" description:"the base url of the service" required:"true"`
	// The service port
	Port string `short:"p" long:"port" description:"the listening port" required:"true"`
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

// Sumd repsents the checksum service
type Sumd struct {
	// the service args
	Args *Args
	// the release cache
	Cache *map[string]CachedRelease
	// the cache update ticker
	Ticker *time.Ticker
}

// Constructor
func NewSumd() *Sumd {
	sumd = &Sumd{
		Args:  &Args{},
		Cache: &map[string]CachedRelease{},
	}

	// update release cache every 2 minutes
	sumd.Ticker = time.NewTicker(time.Minute * 10)
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

	return sumd
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
		return nil, fmt.Errorf("failed to open release file: %s", err)
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
func (sumd *Sumd) verify(metadata *map[string]interface{}, releaseDir string) (map[string]interface{}, error) {
	product, hasProduct := (*metadata)["product"].(string)
	version, hasVersion := (*metadata)["version"].(string)
	releaseFile, hasFile := (*metadata)["file"].(string)
	distributionSum, hasdistributionSum := (*metadata)["checksum"].(string)
	if !hasProduct {
		return nil, errors.New("invalid metadata structure, missing 'product' required field")
	}
	if !hasVersion {
		return nil, errors.New("invalid metadata structure, missing 'version' required field")
	}
	if !hasFile {
		return nil, errors.New("invalid metadata structure, missing 'file' required field")
	}
	if !hasdistributionSum {
		return nil, errors.New("invalid metadata structure, missing 'checksum' required field")
	}

	file, err := sumd.getReleaseFile(version, product, releaseFile, releaseDir)
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
		"distributionchecksum": distributionSum,
	}

	if distributionSum == releaseSum {
		payload["verified"] = true
		key, err := sumd.cacheRelease(product, version, releaseFile)
		if err != nil {
			return nil, fmt.Errorf("failed to cache release file: %s", err)
		}
		url := sumd.formUrl(key, releaseFile)
		payload["download"] = url
	} else {
		payload["verified"] = false
		payload["error"] = "Checksum verification failed for the requested file, the download has been aborted for your safety. Report this issue to the maintainer or software dev team."
	}

	return payload, nil
}
