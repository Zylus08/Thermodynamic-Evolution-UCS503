package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Data Models
// ─────────────────────────────────────────────────────────────────────────────

// HealthResponse is the shape returned by GET /status.
type HealthResponse struct {
	Status  string `json:"status"`
	Module  string `json:"module"`
	Version string `json:"version"`
}

// Deliverable holds metadata for a single uploaded file.
// This lives in memory until we wire up PostgreSQL in Phase 2.
type Deliverable struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	Date      string `json:"date"`
	Summary   string `json:"summary"`
	Filename  string `json:"filename"`   // original filename e.g. "deck.pptx"
	FileURL   string `json:"file_url"`   // served path  e.g. "/uploads/1715_deck.pptx"
	UploadedAt string `json:"uploaded_at"` // ISO-8601 timestamp
}

// UploadResponse is the shape returned by POST /upload.
type UploadResponse struct {
	Success  bool        `json:"success"`
	Message  string      `json:"message"`
	Data     Deliverable `json:"data"` // embed the full deliverable so React gets every field
}

// ─────────────────────────────────────────────────────────────────────────────
// In-memory store (thread-safe)
// Replace with PostgreSQL calls in Phase 2 — the handler signatures stay the same.
// ─────────────────────────────────────────────────────────────────────────────

var (
	deliverables []Deliverable  // the slice that acts as our temporary database
	mu           sync.RWMutex   // protects deliverables from concurrent access
	nextID       = 1            // monotonically increasing primary key
)

// ─────────────────────────────────────────────────────────────────────────────
// CORS helper — applied to every response so the Vite dev server can reach us.
// In production Nginx handles CORS at the proxy layer.
// ─────────────────────────────────────────────────────────────────────────────

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// ─────────────────────────────────────────────────────────────────────────────
// Handler: POST /upload
// ─────────────────────────────────────────────────────────────────────────────

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	// Handle CORS pre-flight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// ── 1. Parse multipart body (max 50 MB in RAM) ───────────────────────────
	const maxMemory = 50 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		log.Printf("[ERROR] ParseMultipartForm: %v", err)
		http.Error(w, "Bad Request: cannot parse form", http.StatusBadRequest)
		return
	}

	// ── 2. Pull the file out of the form ─────────────────────────────────────
	formFile, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("[ERROR] FormFile: %v", err)
		http.Error(w, "Bad Request: missing file field", http.StatusBadRequest)
		return
	}
	defer formFile.Close()

	log.Printf("[INFO] Received: %s (%d bytes)", header.Filename, header.Size)

	// ── 3. Extract metadata fields ────────────────────────────────────────────
	title   := r.FormValue("title")
	version := r.FormValue("version")
	date    := r.FormValue("date")
	summary := r.FormValue("summary")

	// ── 4. Ensure upload directory exists ─────────────────────────────────────
	uploadDir := "./uploads"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		log.Printf("[ERROR] MkdirAll: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// ── 5. Build a collision-proof filename ───────────────────────────────────
	// Format: <unix_ms>_original-name.ext  →  1715629123456_deck.pptx
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	savedName := fmt.Sprintf("%d_%s", timestamp, header.Filename)
	savedPath := filepath.Join(uploadDir, savedName)

	// ── 6. Stream bytes to disk ───────────────────────────────────────────────
	dest, err := os.Create(savedPath)
	if err != nil {
		log.Printf("[ERROR] os.Create: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer dest.Close()

	written, err := io.Copy(dest, formFile)
	if err != nil {
		log.Printf("[ERROR] io.Copy: %v", err)
		http.Error(w, "Internal Server Error: write failed", http.StatusInternalServerError)
		return
	}
	log.Printf("[INFO] Saved %d bytes → %s", written, savedPath)

	// ── 7. Build the Deliverable record and store it ──────────────────────────
	fileURL := fmt.Sprintf("/uploads/%s", savedName)

	mu.Lock()
	d := Deliverable{
		ID:         nextID,
		Title:      title,
		Version:    version,
		Date:       date,
		Summary:    summary,
		Filename:   header.Filename,
		FileURL:    fileURL,
		UploadedAt: time.Now().UTC().Format(time.RFC3339),
	}
	deliverables = append(deliverables, d)
	nextID++
	mu.Unlock()

	// ── 8. Return the full deliverable in the response ────────────────────────
	resp := UploadResponse{
		Success: true,
		Message: "Archive committed to storage matrix.",
		Data:    d,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[ERROR] encode response: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Handler: GET /deliverables
// Returns all uploaded deliverables as a JSON array.
// ─────────────────────────────────────────────────────────────────────────────

func deliverablesHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	mu.RLock()
	// Return an empty JSON array instead of null when there are no uploads yet
	snapshot := make([]Deliverable, len(deliverables))
	copy(snapshot, deliverables)
	mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		log.Printf("[ERROR] encode deliverables: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Main — router + server bootstrap
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	mux := http.NewServeMux()

	// GET /status — health check
	// (Nginx rewrites /api/status → /status inside the container)
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := HealthResponse{Status: "SYS.INITIALIZE", Module: "Go Backend", Version: "1.0.0"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("[ERROR] encode health: %v", err)
		}
	})

	// POST /upload — receive a file + metadata, save to disk, store record
	mux.HandleFunc("/upload", uploadHandler)

	// GET /deliverables — return the in-memory archive as JSON
	mux.HandleFunc("/deliverables", deliverablesHandler)

	// GET /uploads/<filename> — static file server for uploaded files
	// http.StripPrefix removes "/uploads/" before FileServer looks in ./uploads/
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	port := ":8080"
	log.Printf("[SYS] Go API listening on %s", port)
	log.Printf("[SYS] Routes: GET /status | POST /upload | GET /deliverables | GET /uploads/<file>")
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("[FATAL] %v", err)
	}
}
