package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	dir := flag.String("dir", "", "directory containing input video files")
	addr := flag.String("addr", ":8080", "HTTP server address")
	flag.Parse()

	if *dir == "" {
		flag.Usage()
		log.Fatal("dir is required")
	}

	http.HandleFunc("/", newHandler(*dir))
	log.Printf("listening on %s, serving files from %s", *addr, *dir)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}
