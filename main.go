package sse

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"time"
)

var audioStream = make(chan []byte)

func sseHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting new SSE connection")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	fmt.Fprintf(w, "retry: 1000\n\n") // Tell client to retry every second if connection is lost
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
    		select {
    			case <-r.Context().Done():
        			log.Println("Client disconnected via context")
        			return
			case <-ticker.C:
        			fmt.Fprintf(w, ": keep-alive\n\n")
        			flusher.Flush()
    			case data, ok := <-audioStream:
        			if !ok {
            				log.Println("audioStream closed")
            				return
        			}
        			if len(data) > 0 {
            				fmt.Fprintf(w, "data: %s\n\n", base64.StdEncoding.EncodeToString(data))
            				flusher.Flush()
            				log.Println("SSE_HANDLER_LOOP: Successfully flushed data to client.")
        			}
    		}
	}

}

func SendBytes(bytes []byte) {
	log.Printf("SSE_SEND: Attempting to send %d bytes to audioStream", len(bytes))
	audioStream <- bytes
	log.Println("SSE_SEND: Successfully sent to audioStream")
}

func Start() {
	http.HandleFunc("/sse", sseHandler)
}
