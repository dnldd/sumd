package main

import (
	"log"
	"net/http"

	flags "github.com/jessevdk/go-flags"
)

// run with ./sumd --reldir=rel --baseurl=http://127.0.0.1 --pi=https://127.0.0.1:59374 --port=:55650
var sumd *Sumd

func main() {
	args := &Args{}
	_, err := flags.Parse(args)
	if err != nil {
		panic(err)
	}

	sumd, err = NewSumd(args)
	if err != nil {
		panic(err)
	}

	log.Println(">>> started sumd on", sumd.Args.Port)
	log.Fatal(http.ListenAndServe(sumd.Args.Port, CreateRoutes()))
}
