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

func audioInputHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	bodyBytes := make([]byte, 1024) // Adjust buffer size as needed
	n, err := r.Body.Read(bodyBytes)
	if err != nil && err.Error() != "EOF" {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	if n > 0 {
		// Send to all connected SSE clients
		// A copy is made to avoid race conditions if audioStream was buffered
		// and multiple goroutines were reading from r.Body concurrently (not the case here but good practice)
		dataToSend := make([]byte, n)
		copy(dataToSend, bodyBytes[:n])
		audioStream <- dataToSend
		fmt.Fprintln(w, "Audio data received and broadcasted")
	} else {
		fmt.Fprintln(w, "No audio data received")
	}
}

func main() {
	http.HandleFunc("/events", sseHandler)
	http.HandleFunc("/audio", audioInputHandler)

	// For demonstration: periodically send some dummy audio data
	// In a real application, you'd get this from an actual audio source
	go func() {
		for {
			time.Sleep(5 * time.Second)
			dummyData := []byte(fmt.Sprintf("Audio chunk at %s", time.Now().Format(time.RFC3339)))
			log.Printf("Broadcasting dummy data: %s\n", string(dummyData))
			// Non-blocking send to avoid deadlocking if no clients are connected
			// However, the current sseHandler design with a shared audioStream channel
			// means if no client is listening, this will block.
			// For true non-blocking, each client would need its own channel,
			// and the audioInputHandler would iterate over active clients.
			// Or, use a buffered audioStream, but that can lead to stale data.
			// The current approach ensures all connected clients get all data,
			// but requires at least one client to be listening to progress.
			select {
			case audioStream <- dummyData:
			default:
				log.Println("No clients connected, dummy data not sent.")
			}
		}
	}()

	log.Println("Server starting on :6971...")
	if err := http.ListenAndServe(":6971", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
