package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// WriteErrorCodeResponse convenience func for creating a json error response
func WriteErrorCodeResponse(writer *http.ResponseWriter, code int, detail string) {
	errorBody := map[string]interface{}{}
	errorBody["errors"] = map[string]string{"msg": detail}
	detailBytes, _ := json.Marshal(errorBody)
	(*writer).Header().Set("Content-Type", "application/json")
	(*writer).WriteHeader(code)
	fmt.Fprintln(*writer, string(detailBytes))
}

// WriteResponse convenience func for creating a json response
func WriteResponse(writer *http.ResponseWriter, detail string) {
	detailjson, _ := json.Marshal(map[string]string{"response": detail})
	(*writer).Header().Set("Content-Type", "application/json")
	fmt.Fprintln(*writer, string(detailjson))
}

// WriteObject convenience func for writing a json object to a request
func WriteObject(writer *http.ResponseWriter, code int, detailjson *[]byte) {
	(*writer).Header().Set("Content-Type", "application/json")
	(*writer).WriteHeader(code)
	fmt.Fprintln(*writer, string(*detailjson))
}

// AddCORSHeaders sets CORS headers
func AddCORSHeaders(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Accept-Encoding, Accept-Language, Access-Control-Request-Headers, Access-Control-Request-Method, Connection, Host, Origin, Referer, User-Agent, Authorization, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.Header().Set("Access-Control-Allow-Credentials", "false")

		fn(w, r)
	}
}

// Options is a HundlerFunc wrapper for handling OPTIONS requests
func Options(w http.ResponseWriter, r *http.Request) {
}
