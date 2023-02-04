package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	http.Handle("/", http.FileServer(http.Dir("./dist")))
	server := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 3 * time.Second,
	}
	log.Print("Listening on :8080...")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
