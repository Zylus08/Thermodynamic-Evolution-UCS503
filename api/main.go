package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// HealthResponse defines the structure for our health-check endpoint
type HealthResponse struct {
	Status  string `json:"status"`
	Module  string `json:"module"`
	Version string `json:"version"`
}

func main() {
	// Using standard library mux for a lightweight router
	mux := http.NewServeMux()

	// GET /status endpoint
	// Note: Nginx rewrites /api/status to /status
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := HealthResponse{
			Status:  "SYS.INITIALIZE",
			Module:  "Go Backend",
			Version: "1.0.0",
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[ERROR] Failed to encode health response: %v", err)
		}
	})

	// Start server on internal port 8080
	port := ":8080"
	log.Printf("[SYS] Starting Go API on internal port %s...", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("[FATAL] Server failed to start: %v", err)
	}
}
