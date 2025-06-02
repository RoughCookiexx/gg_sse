package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

var audioStream = make(chan []byte)

func sseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "retry: 1000\n\n") // Tell client to retry every second if connection is lost

	clientChan := make(chan []byte)
	go func() {
		for data := range audioStream { // This will block until new data is sent to audioStream
			clientChan <- data
		}
	}()

	defer func() {
		// Optional: clean up client-specific resources if any
		log.Println("Client disconnected")
	}()

	for {
		select {
		case <-r.Context().Done():
			log.Println("Client disconnected via context")
			return
		case audioData := <-clientChan:
			if len(audioData) > 0 {
				fmt.Fprintf(w, "data: %s\n\n", string(audioData)) // Assuming audio bytes can be represented as string for simplicity
				flusher.Flush()
			}
		}
	}
}

func SendBytes(bytes []byte) {
	audioStream <- bytes
}

func Start() {
	http.HandleFunc("/sse", sseHandler)
}
