package main

import (
	"bytes"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	flags "github.com/btcsuite/go-flags"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/politeia/politeiad/api/v1"
	"github.com/decred/politeia/politeiad/api/v1/identity"
	"github.com/decred/politeia/util"
)

// the cli args for sumdemo
type Args struct {
	// pi's endpoint
	Pi string `long:"pi" description:"pi's endpoint" required:"true"`
	// sumd's endpoint
	Sumd string `long:"sumd" description:"sumd's endpoint" required:"true"`
	// the rpc user
	RPCUser string `long:"rpcuser" description:"the rpc user" required:"true"`
	// the rpc pass
	RPCPass string `long:"rpcpass" description:"the rpc pass" required:"true"`
	// the flag to run failure case
	Fail bool `long:"fail" description:"run failure case"`
}

// For success case: ./sumdemo --pi=https://127.0.0.1:59374 --sumd=http://127.0.0.1:55650 --rpcuser=zY2Ylw7mDoh/IlpyvTp4KG3mpWA= --rpcpass=2hePPg5Qo6ywJIdmrvRCvutsB8I=

// For failure case: ./sumdemo --pi=https://127.0.0.1:59374 --sumd=http://127.0.0.1:55650 --rpcuser=zY2Ylw7mDoh/IlpyvTp4KG3mpWA= --rpcpass=2hePPg5Qo6ywJIdmrvRCvutsB8I= --fail

var (
	// args
	args = &Args{}
	// verify endpoint
	verifyRoute = "/verify"
	// the http client
	client = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	// the full identity of the client
	fi *identity.FullIdentity
	// the public identity of pi
	pipi *identity.PublicIdentity
	// the censorship record
	censorshipRecord *v1.CensorshipRecord
	// the checksum metadata records
	checksumRecords *[]v1.MetadataStream
	// the release file checksum payload
	checksumPayload *map[string]interface{}
)

// prettyPrint json pretty printer
func prettyPrint(src *[]byte) []byte {
	dst := &bytes.Buffer{}
	json.Indent(dst, *src, "", "  ")
	return dst.Bytes()
}

// run with:

// postNewRecord creates a new release record on pi
func postRecord() error {
	// create record payload
	var checksumRecord map[string]interface{}
	if !args.Fail {
		// payload for success case
		checksumRecord = map[string]interface{}{
			"checksum": "5eccbbea3f91c9646a6c9d1462137b68e9a81b8d860b3855011e9fe6dcc3f280",
			"product":  "mounty",
			"version":  "1.7",
			"file":     "mounty.dmg",
		}
	} else {
		// payload for failure case
		checksumRecord = map[string]interface{}{
			"checksum": "5eccbbea3f91c9646a6c9d1462137b68e9a81b8d860b3855011e9fe6dcc3f281",
			"product":  "mounty",
			"version":  "1.7",
			"file":     "mounty.dmg",
		}
	}
	checksumRecordBytes, err := json.Marshal(checksumRecord)
	if err != nil {
		return err
	}
	metadata := v1.MetadataStream{
		ID:      1,
		Payload: string(checksumRecordBytes),
	}
	name := "mounty-v1.7.md"
	mime, digest, content, err := util.LoadFile(name)
	if err != nil {
		return err
	}
	file := v1.File{
		Name:    name,
		MIME:    mime,
		Digest:  digest,
		Payload: content,
	}
	payload := v1.NewRecord{
		Challenge: hex.EncodeToString(fi.Public.Key[:]),
		//hex.EncodeToString(challenge),
		Metadata: []v1.MetadataStream{metadata},
		Files:    []v1.File{file},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// post new pi record
	req, err := http.NewRequest("POST", args.Pi+v1.NewRecordRoute,
		bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body := util.ConvertBodyToByteArray(resp.Body, false)
	reply := &v1.NewRecordReply{}
	err = json.Unmarshal(body, reply)
	if err != nil {
		return err
	}

	// verify server response & client challenge
	sigBytes := []byte(reply.CensorshipRecord.Signature)
	var sig [identity.SignatureSize]byte
	copy(sig[:], sigBytes)
	if pipi.VerifyMessage(body, sig) {
		return errors.New("message verification failed")
	}
	err = util.VerifyChallenge(pipi, fi.Public.Key[:], reply.Response)
	if err != nil {
		return err
	}
	censorshipRecord = &reply.CensorshipRecord
	log.Printf(">>> record created:\n%s\n", prettyPrint(&body))
	return nil
}

// publishRecord updates a record unvetted status to be publicly visible
func publishRecord() error {
	// create record update payload
	payload := v1.SetUnvettedStatus{
		Challenge: hex.EncodeToString(fi.Public.Key[:]),
		Token:     censorshipRecord.Token,
		Status:    v1.RecordStatusPublic,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// post vetting status update
	req, err := http.NewRequest("POST", args.Pi+v1.SetUnvettedStatusRoute,
		bytes.NewReader(payloadBytes))
	req.SetBasicAuth(args.RPCUser, args.RPCPass)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body := util.ConvertBodyToByteArray(resp.Body, false)
	reply := &v1.SetUnvettedStatusReply{}
	err = json.Unmarshal(body, reply)
	if err != nil {
		return err
	}

	// verify client challenge
	err = util.VerifyChallenge(pipi, fi.Public.Key[:], reply.Response)
	if err != nil {
		return err
	}
	log.Printf(">>> record updated, now publicly visible:\n%s\n", prettyPrint(&body))
	return nil
}

// fetchChecksumRecords fetch checksum metadata from a publish pi record
// func fetchChecksumRecords() error {
// 	// create record retrieval payload
// 	payload := v1.GetVetted{
// 		Challenge: hex.EncodeToString(fi.Public.Key[:]),
// 		Token:     censorshipRecord.Token,
// 	}
// 	payloadBytes, err := json.Marshal(payload)
// 	if err != nil {
// 		return err
// 	}
//
// 	// request pi record
// 	req, err := http.NewRequest("POST", args.Pi+v1.GetVettedRoute,
// 		bytes.NewReader(payloadBytes))
// 	if err != nil {
// 		return err
// 	}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()
// 	body := util.ConvertBodyToByteArray(resp.Body, false)
// 	reply := &v1.GetVettedReply{}
// 	err = json.Unmarshal(body, reply)
// 	if err != nil {
// 		return err
// 	}
// 	sigBytes := []byte(reply.Record.CensorshipRecord.Signature)
// 	var sig [identity.SignatureSize]byte
// 	copy(sig[:], sigBytes)
//
// 	// verify pi response & client challenge
// 	if pipi.VerifyMessage(body, sig) {
// 		return errors.New("message verification failed")
// 	}
// 	err = util.VerifyChallenge(pipi, fi.Public.Key[:], reply.Response)
// 	if err != nil {
// 		return err
// 	}
// 	checksumRecords = &reply.Record.Metadata
// 	recordBytes, err := json.Marshal(*checksumRecords)
// 	if err != nil {
// 		return err
// 	}
// 	log.Printf(">>> pi record retrieved, release information obtained from metadata:\n%s\n", prettyPrint(&recordBytes))
// 	return nil
// }

// VerifyRelease asserts the integrity of a release file
func VerifyRelease(payload *map[string]interface{}) error {
	payloadBytes, err := json.Marshal(*payload)
	if err != nil {
		return err
	}
	// verify release info with sumd
	log.Println(">>> verifying release information with sumd...")
	req, err := http.NewRequest("POST", args.Sumd+verifyRoute,
		bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body := util.ConvertBodyToByteArray(resp.Body, false)
	reply := &map[string]interface{}{}
	err = json.Unmarshal(body, reply)
	if err != nil {
		return err
	}
	checksumPayload = reply
	log.Printf(">>> results:\n%s\n", prettyPrint(&body))
	return nil
}

func DownloadReleaseFile() error {
	verified, hasVerified := (*checksumPayload)["verified"].(bool)
	if hasVerified {
		if verified {
			// download release file
			log.Println(">>> release file verified, downloading...")
			url, _ := (*checksumPayload)["download"].(string)
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return err
			}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body := util.ConvertBodyToByteArray(resp.Body, false)
			log.Printf(">>> %d bytes downloaded.", len(body))
			return nil
		}
		log.Println(">>> failed to verify release file, aborting download.")
		detail, _ := (*checksumPayload)["error"].(string)
		return errors.New(detail)
	}
	return errors.New("malformed checksum payload")
}

func main() {
	spew.Config.Indent = "\t"
	_, err := flags.Parse(args)
	if err == nil {
		fi, err = identity.LoadFullIdentity("../identity.json")
		if err != nil {
			log.Println(err)
		}
		pipi, err = util.RemoteIdentity(true, args.Pi, "")
		if err != nil {
			log.Println(err)
		}
		err = postRecord()
		if err != nil {
			log.Println(err)
		}
		err = publishRecord()
		if err != nil {
			log.Println(err)
		}

		verifyPayload := map[string]interface{}{
			"token":   censorshipRecord.Token,
			"product": "mounty",
			"version": "1.7",
			"file":    "mounty.dmg",
		}

		err := VerifyRelease(&verifyPayload)
		if err != nil {
			log.Println(err)
		}

		err = DownloadReleaseFile()
		if err != nil {
			log.Println(err)
		}
	}
}
