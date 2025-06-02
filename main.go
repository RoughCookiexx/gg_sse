package sse

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
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
	flusher.Flush()
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "retry: 1000\n\n") // Tell client to retry every second if connection is lost
	flusher.Flush()

	clientChan := make(chan []byte)
	go func() {
	    	log.Println("SSE_GOROUTINE: Started. Ranging over audioStream.")
    		for data := range audioStream {
       			log.Printf("SSE_GOROUTINE: Got %d bytes from audioStream. Attempting to send to clientChan.", len(data))
        		select {
        			case clientChan <- data:
            				log.Printf("SSE_GOROUTINE: Successfully sent %d bytes to clientChan.", len(data))
        			case <-r.Context().Done(): // If the client request is cancelled
            				log.Println("SSE_GOROUTINE: Client context done. Stopping.")
           	 			return
        		}
    		}
		log.Println("SSE_GOROUTINE: audioStream was closed or loop ended.")
	}()

	defer func() {
        	close(clientChan)
		log.Println("Client disconnected")
	}()

	for {
		select {
		case <-r.Context().Done():
			log.Println("Client disconnected via context")
			return
		case audioData := <-clientChan:
			if len(audioData) > 0 {
				fmt.Fprintf(w, "data: %s\n\n", base64.StdEncoding.EncodeToString(audioData))
				flusher.Flush()
				log.Println("SSE_HANDLER_LOOP: Successfully flushed data to client/Nginx.")
			}
    			flusher.Flush()
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
