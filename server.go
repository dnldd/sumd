package main

import (
	"log"
	"net/http"

	flags "github.com/jessevdk/go-flags"
)

// run with ./sumd --reldir=rel --baseurl=http://127.0.0.1 --port=:55650
var sumd *Sumd

func main() {
	sumd = NewSumd()
	_, err := flags.Parse(sumd.Args)
	if err != nil {
		panic(err)
	}

	log.Println(">>> started sumd on", sumd.Args.Port)
	log.Fatal(http.ListenAndServe(sumd.Args.Port, CreateRoutes()))
}
