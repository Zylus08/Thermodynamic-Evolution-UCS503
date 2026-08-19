package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
<<<<<<< HEAD
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
)


// HealthResponse defines the structure for our health-check endpoint
=======
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Data Models
// ─────────────────────────────────────────────────────────────────────────────

// HealthResponse is the shape returned by GET /status.
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae
type HealthResponse struct {
	Status  string `json:"status"`
	Module  string `json:"module"`
	Version string `json:"version"`
}

<<<<<<< HEAD
// ArchiveEntry represents a stored upload's metadata
type ArchiveEntry struct {
	Filename     string `json:"filename"`
	OriginalName string `json:"originalName"`
	Title        string `json:"title"`
	Version      string `json:"version"`
	Summary      string `json:"summary"`
	UploadedAt   string `json:"uploadedAt"`
	URL          string `json:"url"`
}

var (
	archivesFile = "archives.json"
	uploadsDir   = "uploads"
	mu           sync.Mutex
	adminPasskey = func() string {
		if v := os.Getenv("ADMIN_PASSKEY"); v != "" {
			return v
		}
		if v := os.Getenv("API_TOKEN"); v != "" {
			return v
		}
		return ""
	}()

	// S3 config (optional)
	s3Bucket string
	s3Region string
	s3Client *s3.Client
	useS3    bool
	// presign expiration (default 24h)
	presignExpire = 24 * time.Hour

	// Secure admin sessions
	sessionTTL     = 8 * time.Hour
	sessionMu      sync.Mutex
	sessionStore   = map[string]time.Time{}
	sessionCookie  = "ucs503_session"

	// Postgres
	dbPool *pgxpool.Pool
	useDB  bool
)

func randomSessionToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func hasValidSession(r *http.Request) bool {
	if r == nil {
		return false
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return false
	}

	sessionMu.Lock()
	defer sessionMu.Unlock()

	expiresAt, ok := sessionStore[cookie.Value]
	if !ok {
		return false
	}
	if time.Now().After(expiresAt) {
		delete(sessionStore, cookie.Value)
		return false
	}
	return true
}

func constantTimeEquals(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func ensureUploadsDir() error {
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		return os.MkdirAll(uploadsDir, 0o755)
	}
	return nil
}

func loadArchives() ([]ArchiveEntry, error) {
	if useDB && dbPool != nil {
		// read from Postgres
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rows, err := dbPool.Query(ctx, "SELECT filename, original_name, title, version, summary, uploaded_at, url FROM archives ORDER BY uploaded_at DESC")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var entries []ArchiveEntry
		for rows.Next() {
			var e ArchiveEntry
			var uploaded time.Time
			if err := rows.Scan(&e.Filename, &e.OriginalName, &e.Title, &e.Version, &e.Summary, &uploaded, &e.URL); err != nil {
				return nil, err
			}
			e.UploadedAt = uploaded.Format(time.RFC3339)
			entries = append(entries, e)
		}
		return entries, nil
	}

	mu.Lock()
	defer mu.Unlock()

	if _, err := os.Stat(archivesFile); os.IsNotExist(err) {
		return []ArchiveEntry{}, nil
	}

	f, err := os.Open(archivesFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []ArchiveEntry
	if err := json.NewDecoder(f).Decode(&entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func saveArchives(entries []ArchiveEntry) error {
	if useDB && dbPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Replace contents by deleting old rows and inserting current list in order
		_, err := dbPool.Exec(ctx, "CREATE TABLE IF NOT EXISTS archives (filename text PRIMARY KEY, original_name text, title text, version text, summary text, uploaded_at timestamptz, url text)")
		if err != nil {
			return err
		}
		// Upsert each entry
		for _, e := range entries {
			uploaded, _ := time.Parse(time.RFC3339, e.UploadedAt)
			_, err := dbPool.Exec(ctx, "INSERT INTO archives (filename, original_name, title, version, summary, uploaded_at, url) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (filename) DO UPDATE SET original_name=EXCLUDED.original_name, title=EXCLUDED.title, version=EXCLUDED.version, summary=EXCLUDED.summary, uploaded_at=EXCLUDED.uploaded_at, url=EXCLUDED.url",
				e.Filename, e.OriginalName, e.Title, e.Version, e.Summary, uploaded, e.URL)
			if err != nil {
				return err
			}
		}
		return nil
	}

	mu.Lock()
	defer mu.Unlock()

	f, err := os.Create(archivesFile)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func authorized(r *http.Request) bool {
	// Accept X-API-Token or Authorization: Bearer <token>
	if adminPasskey == "" {
		return false
	}
	if hasValidSession(r) {
		return true
	}
	if t := r.Header.Get("X-API-Token"); t != "" {
		return constantTimeEquals(t, adminPasskey)
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		const prefix = "Bearer "
		if len(auth) > len(prefix) && strings.HasPrefix(auth, prefix) {
			return constantTimeEquals(auth[len(prefix):], adminPasskey)
		}
	}
	return false
}

func main() {
	// Initialize optional S3 client if environment configured
	s3Bucket = os.Getenv("S3_BUCKET")
	s3Region = os.Getenv("S3_REGION")
	if s3Bucket != "" {
		useS3 = true
		cfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			log.Printf("[WARN] Unable to load AWS config: %v — falling back to local disk", err)
			useS3 = false
		} else {
			s3Client = s3.NewFromConfig(cfg)
			log.Printf("[SYS] S3 enabled for bucket %s (region %s)", s3Bucket, s3Region)
		}
	}

	// Initialize optional Postgres if DATABASE_URL provided
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, databaseURL)
		if err != nil {
			log.Printf("[WARN] Unable to connect to Postgres: %v — using local JSON storage", err)
			useDB = false
		} else {
			dbPool = pool
			useDB = true
			log.Printf("[SYS] Postgres enabled for metadata storage")
			// ensure migrations table & run migrations
			if err := ensureMigrationsTable(dbPool); err != nil {
				log.Printf("[WARN] unable to ensure migrations table: %v", err)
			}
			if err := runMigrations(dbPool); err != nil {
				log.Printf("[WARN] migrations failed: %v", err)
			}
		}
	}

	log.Printf("[AUTH] adminPasskey configured=%t", adminPasskey != "")

	// Read presign TTL if set (in hours)
	if v := os.Getenv("S3_PRESIGN_EXPIRE_HOURS"); v != "" {
		if h, err := time.ParseDuration(v + "h"); err == nil {
			presignExpire = h
			log.Printf("[SYS] S3 presign TTL set to %v", presignExpire)
		}
	}

	mux := http.NewServeMux()

	loginHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Passkey string `json:"passkey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			if err := r.ParseForm(); err == nil {
				payload.Passkey = r.FormValue("passkey")
			}
		}
		passkey := strings.TrimSpace(payload.Passkey)
		expectedKey := strings.TrimSpace(adminPasskey)
		log.Printf("[AUTH] incoming=%q expected=%q length_in=%d length_exp=%d", passkey, expectedKey, len(passkey), len(expectedKey))
		if passkey == "" || !constantTimeEquals(passkey, expectedKey) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := randomSessionToken()
		sessionMu.Lock()
		sessionStore[token] = time.Now().Add(sessionTTL)
		sessionMu.Unlock()

		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    token,
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
			Secure:   r.TLS != nil,
			MaxAge:   int(sessionTTL.Seconds()),
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "authenticated"})
	}
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/api/login", loginHandler)

	logoutHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
			sessionMu.Lock()
			delete(sessionStore, cookie.Value)
			sessionMu.Unlock()
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
			SameSite: http.SameSiteLaxMode,
			Secure:   r.TLS != nil,
		})
		w.WriteHeader(http.StatusOK)
	}
	mux.HandleFunc("/logout", logoutHandler)
	mux.HandleFunc("/api/logout", logoutHandler)

	// Health endpoint
	statusHandler := func(w http.ResponseWriter, r *http.Request) {
=======
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
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
<<<<<<< HEAD
		response := HealthResponse{Status: "SYS.INITIALIZE", Module: "Go Backend", Version: "1.0.0"}
		_ = json.NewEncoder(w).Encode(response)
	}
	mux.HandleFunc("/status", statusHandler)
	mux.HandleFunc("/api/status", statusHandler)

	// Upload endpoint (requires an authenticated admin session)
	uploadHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if !authorized(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
=======
		resp := HealthResponse{Status: "SYS.INITIALIZE", Module: "Go Backend", Version: "1.0.0"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("[ERROR] encode health: %v", err)
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae
		}

<<<<<<< HEAD
		// Ensure uploads dir (only necessary for local fallback)
		if !useS3 {
			if err := ensureUploadsDir(); err != nil {
				http.Error(w, "Server error: unable to create uploads directory", http.StatusInternalServerError)
				return
			}
		}

		// Limit to reasonable size handled by form parse
		if err := r.ParseMultipartForm(250 << 20); err != nil { // 250MB total
			http.Error(w, "Invalid multipart/form-data", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Missing file in request", http.StatusBadRequest)
			return
		}
		defer file.Close()

		title := r.FormValue("title")
		version := r.FormValue("version")
		summary := r.FormValue("summary")

		// Read a small sample to detect content type
		sample := make([]byte, 512)
		n, _ := file.Read(sample)
		detected := http.DetectContentType(sample[:n])

		// Allowed extensions and mime types
		allowedExts := map[string]bool{
			".pdf":  true,
			".zip":  true,
			".rar":  true,
			".md":   true,
			".txt":  true,
			".ppt":  true,
			".pptx": true,
		}
		allowedMimes := map[string]bool{
			"application/pdf": true,
			"application/zip": true,
			"application/x-rar-compressed": true,
			"text/markdown": true,
			"text/plain": true,
			"application/vnd.ms-powerpoint": true,
			"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
		}

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if !allowedExts[ext] {
			// allow some types by detected mime as well
			if !allowedMimes[detected] {
				http.Error(w, "File type not allowed", http.StatusBadRequest)
				return
			}
		}

		// Enforce per-file size limit (100MB)
		const maxFileSize = 100 << 20
		// recompose reader (we already consumed sample)
		reader := io.MultiReader(bytes.NewReader(sample[:n]), file)

		// Build a safe filename using timestamp + original name
		ts := time.Now().UTC().Format("20060102T150405Z")
		safeName := ts + "__" + filepath.Base(header.Filename)
		outPath := filepath.Join(uploadsDir, safeName)

		if useS3 {
			// Upload to S3 using the reader (with size limit)
			limited := io.LimitReader(reader, maxFileSize+1)
			ctx := context.Background()
			_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
				Bucket:      &s3Bucket,
				Key:         &safeName,
				Body:        limited,
				ContentType: &detected,
			})
			if err != nil {
				http.Error(w, "Server error: unable to upload to S3", http.StatusInternalServerError)
				return
			}
			// No way to detect oversize easily after PutObject; rely on S3 limits or pre-check Content-Length if client provides it
		} else {
			out, err := os.Create(outPath)
			if err != nil {
				http.Error(w, "Server error: unable to save file", http.StatusInternalServerError)
				return
			}
			defer out.Close()

			// Copy with limit to detect oversize
			limited := io.LimitReader(reader, maxFileSize+1)
			written, err := io.Copy(out, limited)
			if err != nil {
				http.Error(w, "Server error: unable to write file", http.StatusInternalServerError)
				return
			}
			if written > maxFileSize {
				// remove partial file
				_ = os.Remove(outPath)
				http.Error(w, "File exceeds maximum allowed size (100MB)", http.StatusRequestEntityTooLarge)
				return
			}
		}

		entry := ArchiveEntry{
			Filename:     safeName,
			OriginalName: header.Filename,
			Title:        title,
			Version:      version,
			Summary:      summary,
			UploadedAt:   time.Now().UTC().Format(time.RFC3339),
			URL:          "/uploads/" + safeName,
		}

		// If using S3, construct public URL (no presigning)
		if useS3 {
			if s3Region != "" {
				entry.URL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s3Bucket, s3Region, safeName)
			} else {
				entry.URL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s3Bucket, safeName)
			}
		}

		entries, err := loadArchives()
		if err != nil {
			http.Error(w, "Server error: unable to load archives", http.StatusInternalServerError)
			return
		}

		entries = append([]ArchiveEntry{entry}, entries...)
		if err := saveArchives(entries); err != nil {
			http.Error(w, "Server error: unable to save archives", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(entry)
	}
	mux.HandleFunc("/upload", uploadHandler)
	mux.HandleFunc("/api/upload", uploadHandler)

	// List archives (public)
	archivesHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		entries, err := loadArchives()
		if err != nil {
			http.Error(w, "Server error: unable to read archives", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	}
	mux.HandleFunc("/archives", archivesHandler)
	mux.HandleFunc("/api/archives", archivesHandler)

	// Delete single archive (requires an authenticated admin session)
	archiveHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorized(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		filename := r.URL.Query().Get("filename")
		if filename == "" {
			http.Error(w, "filename required", http.StatusBadRequest)
			return
		}
		// remove file (S3 or local)
		if useS3 {
			// delete from S3
			ctx := context.Background()
			_, err := s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s3Bucket, Key: &filename})
			if err != nil {
				http.Error(w, "Unable to remove file from S3", http.StatusInternalServerError)
				return
			}
		} else {
			p := filepath.Join(uploadsDir, filename)
			if err := os.Remove(p); err != nil {
				http.Error(w, "Unable to remove file", http.StatusInternalServerError)
				return
			}
		}

		entries, err := loadArchives()
		if err != nil {
			http.Error(w, "Server error: unable to read archives", http.StatusInternalServerError)
			return
		}
		filtered := make([]ArchiveEntry, 0, len(entries))
		for _, e := range entries {
			if e.Filename != filename {
				filtered = append(filtered, e)
			}
		}
		if err := saveArchives(filtered); err != nil {
			http.Error(w, "Server error: unable to save archives", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
	mux.HandleFunc("/archive", archiveHandler)
	mux.HandleFunc("/api/archive", archiveHandler)

	// Serve uploaded files at /uploads/<file> only when not using S3
	if !useS3 {
		mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadsDir))))
		mux.Handle("/api/uploads/", http.StripPrefix("/api/uploads/", http.FileServer(http.Dir(uploadsDir))))
	}

	port := "8080"
	if v := strings.TrimSpace(os.Getenv("PORT")); v != "" {
		if _, err := strconv.Atoi(v); err == nil {
			port = v
		} else {
			log.Printf("[WARN] Ignoring invalid PORT value %q; using default 8080", v)
		}
	}
	addr := ":" + port
	log.Printf("[SYS] Starting Go API on internal port %s...", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[FATAL] Server failed to start: %v", err)
=======
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
>>>>>>> b9beb5fda5b9c42a36ceb65d6ebade5a89a6a3ae
	}
}

// migrations helpers
func ensureMigrationsTable(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())")
	return err
}

func runMigrations(pool *pgxpool.Pool) error {
	migrationsDir := "migrations"
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		// no migrations dir is fine
		return nil
	}
	// sort files
	var names []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".sql") {
			names = append(names, f.Name())
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	ctx := context.Background()
	for _, name := range names {
		version := name
		// check applied
		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", version).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		// read file
		b, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return err
		}
		// execute
		if _, err := pool.Exec(ctx, string(b)); err != nil {
			return err
		}
		// mark applied
		if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", version); err != nil {
			return err
		}
	}
	return nil
}
