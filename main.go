package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/", newHandler())
	log.Printf("listening on %s", *addr)
	if err := http.ListenAndServe(*addr, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}
