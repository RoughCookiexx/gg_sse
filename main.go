package sse

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

var ( 
	clientStreams = make(map[chan []byte]bool)
	clientsMutex = sync.Mutex{}
)	

func sseHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting new SSE connection")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Println("[ERROR] Streaming unsupported!")
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	clientChan := make(chan []byte)
	
	clientsMutex.Lock()
	clientStreams[clientChan] = true
	clientsMutex.Unlock()
	log.Printf("[INFO] New client connected. Total clients: %d", len(clientStreams))

	defer func() {
		clientsMutex.Lock()
		delete(clientStreams, clientChan)
		close(clientChan)
		clientsMutex.Unlock()
		log.Printf("[INFO] Client disconnected. Total clients: %d", len(clientStreams))
	}()

	keepAliveTicker := time.NewTicker(15 * time.Second)
	defer keepAliveTicker.Stop()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
    		select {
    			case <-r.Context().Done():
        			return
			case <-keepAliveTicker.C:
        			_, err := fmt.Fprintf(w, ": keep-alive\n\n")
				if err != nil {
					log.Printf("[ERROR] Keep-alive write error to client: %v", err)
					return
				}
        			flusher.Flush()
    			case data := <-clientChan:
        			if !ok {
            				log.Println("[ERROR] Could not write to audio stream")
            				return
        			}
        			if len(data) > 0 {
            				_, err := fmt.Fprintf(w, "data: %s\n\n", base64.StdEncoding.EncodeToString(data))
					if err != nil {
						log.Printf("[ERROR] Cannot write to client: %v", err)
						return
					}
            				flusher.Flush()
        			}
    		}
	}

}

func SendBytes(bytes []byte) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	
	if len(clientStreams) == 0 {
		log.Println("[WARM] SendBytes called, but there are no connected clients.")
		return
	}
	
	log.Printf("[INFO] Broadcasting %d bytes to %d clients", len(bytes), len(clientStreams))
	for clientChan := range clientStreams {
		select {
		case clientChan <- bytes:
		default:
			log.Println("[WARN] Dropped message for a client, channel was full.")
		}
	}
}

func Start() {
	http.HandleFunc("/sse", sseHandler)
}
