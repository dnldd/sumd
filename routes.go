package main

// CreateRoutes wires up service routes
import (
	"einheit/cedindex/service"
	"einheit/cedindex/utils"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func CreateRoutes() *mux.Router {
	router := mux.NewRouter()
	router.Methods("OPTIONS").Handler(AddCORSHeaders(Options))
	router.HandleFunc("/verify", service.AddCORSHeaders(VerifyChecksum)).Methods("POST")
	router.HandleFunc("/download/{key}/{file}", service.AddCORSHeaders(GetReleaseFile)).Methods("GET")
	return router
}

// VerifyChecksum endpoint for data integrity verification
func VerifyChecksum(writer http.ResponseWriter, request *http.Request) {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		WriteErrorCodeResponse(&writer, http.StatusInternalServerError, "failed to read request body")
		return
	}

	if len(body) == 0 {
		WriteErrorCodeResponse(&writer, http.StatusBadRequest, "request body is empty")
		return
	}

	data, err := utils.JSONUnmarshal(&body)
	if err != nil {
		service.WriteErrorCodeResponse(&writer, http.StatusBadRequest, "request body is invalid json")
		return
	}

	token, tokenOk := data["token"].(string)
	if !tokenOk {
		WriteErrorCodeResponse(&writer, http.StatusBadRequest, "required 'token' param not found")
		return
	}
	product, productOk := data["product"].(string)
	if !productOk {
		WriteErrorCodeResponse(&writer, http.StatusBadRequest, "required 'product' param not found")
		return
	}
	version, versionOk := data["version"].(string)
	if !versionOk {
		WriteErrorCodeResponse(&writer, http.StatusBadRequest, "required 'version' param not found")
		return
	}
	file, fileOk := data["file"].(string)
	if !fileOk {
		WriteErrorCodeResponse(&writer, http.StatusBadRequest, "required 'file' param not found")
		return
	}

	payload, err := sumd.verify(token, product, version, file)
	if err != nil {
		if err.Error() == "release file not found" {
			WriteErrorCodeResponse(&writer, http.StatusBadRequest, err.Error())
			return
		}
		WriteErrorCodeResponse(&writer, http.StatusInternalServerError, err.Error())
		return
	}

	responseJSON, _ := json.Marshal(payload)
	WriteObject(&writer, http.StatusOK, &responseJSON)
	return
}

// GetReleaseFile start a download for a release file
func GetReleaseFile(writer http.ResponseWriter, request *http.Request) {
	params := mux.Vars(request)
	key := params["key"]
	if key == "" {
		WriteErrorCodeResponse(&writer, http.StatusBadRequest, "required url param 'hash' not found")
		return
	}

	payload, ok := (*sumd.Cache)[key]

	if !ok {
		WriteErrorCodeResponse(&writer, http.StatusNotFound, fmt.Sprintf("%s: release file with supplied key not found"))
		return
	}

	file, err := sumd.getReleaseFile(payload.Version, payload.Product, payload.File, sumd.Args.ReleaseDir)
	if err != nil {
		WriteErrorCodeResponse(&writer, http.StatusNotFound, fmt.Sprintf("%s: file not found"))
		return
	}
	defer file.Close()

	stats, _ := file.Stat()
	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		WriteErrorCodeResponse(&writer, http.StatusInternalServerError, "failed to read file")
		return
	}
	// Reset the read pointer
	file.Seek(0, 0)
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; %s", payload.File))
	writer.Header().Set("Content-Type", http.DetectContentType(buffer))
	writer.Header().Set("Content-Length", strconv.Itoa(int(stats.Size())))
	// stream the file
	io.Copy(writer, file)
}
