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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthResponse defines the structure for our health-check endpoint.
type HealthResponse struct {
	Status  string `json:"status"`
	Module  string `json:"module"`
	Version string `json:"version"`
}

// ArchiveEntry represents a stored upload's metadata.
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

	s3Bucket string
	s3Region string
	s3Client *s3.Client
	useS3    bool

	presignExpire = 24 * time.Hour

	sessionTTL    = 8 * time.Hour
	sessionMu     sync.Mutex
	sessionStore  = map[string]time.Time{}
	sessionCookie = "ucs503_session"

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

func sessionSameSite(r *http.Request) http.SameSite {
	if r != nil && r.TLS != nil {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := map[string]struct{}{
		"http://localhost:5173":     {},
		"http://127.0.0.1:5173":     {},
		"https://zylus08.github.io": {},
	}
	for _, origin := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		origin = strings.TrimSpace(strings.TrimRight(origin, "/"))
		if origin != "" {
			allowedOrigins[origin] = struct{}{}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimRight(r.Header.Get("Origin"), "/")
		if _, allowed := allowedOrigins[origin]; allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Token, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			if origin == "" {
				http.Error(w, "Origin required", http.StatusForbidden)
				return
			}
			if _, allowed := allowedOrigins[origin]; !allowed {
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ensureUploadsDir() error {
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		return os.MkdirAll(uploadsDir, 0o755)
	}
	return nil
}

func loadArchives() ([]ArchiveEntry, error) {
	if useDB && dbPool != nil {
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

		_, err := dbPool.Exec(ctx, "CREATE TABLE IF NOT EXISTS archives (filename text PRIMARY KEY, original_name text, title text, version text, summary text, uploaded_at timestamptz, url text)")
		if err != nil {
			return err
		}

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
		return nil
	}

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
		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", version).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		b, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(b)); err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", version); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	s3Bucket = os.Getenv("S3_BUCKET")
	s3Region = os.Getenv("S3_REGION")
	if s3Bucket != "" {
		useS3 = true
		cfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			log.Printf("[WARN] Unable to load AWS config: %v ? falling back to local disk", err)
			useS3 = false
		} else {
			s3Client = s3.NewFromConfig(cfg)
			log.Printf("[SYS] S3 enabled for bucket %s (region %s)", s3Bucket, s3Region)
		}
	}

	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool, err := pgxpool.New(ctx, databaseURL)
		cancel()
		if err != nil {
			log.Printf("[WARN] Unable to connect to Postgres: %v ? using local JSON storage", err)
			useDB = false
		} else {
			dbPool = pool
			useDB = true
			log.Printf("[SYS] Postgres enabled for metadata storage")
			if err := ensureMigrationsTable(dbPool); err != nil {
				log.Printf("[WARN] unable to ensure migrations table: %v", err)
			}
			if err := runMigrations(dbPool); err != nil {
				log.Printf("[WARN] migrations failed: %v", err)
			}
		}
	}

	if !useS3 {
		if err := ensureUploadsDir(); err != nil {
			log.Printf("[WARN] Unable to create uploads dir: %v", err)
		}
	}

	log.Printf("[AUTH] adminPasskey configured=%t", adminPasskey != "")

	if v := os.Getenv("S3_PRESIGN_EXPIRE_HOURS"); v != "" {
		if h, err := time.ParseDuration(v + "h"); err == nil {
			presignExpire = h
			log.Printf("[SYS] S3 presign TTL set to %v", presignExpire)
		}
	}

	mux := http.NewServeMux()

	statusHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := HealthResponse{Status: "SYS.INITIALIZE", Module: "Go Backend", Version: "1.0.0"}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[ERROR] encode health: %v", err)
		}
	}
	mux.HandleFunc("/status", statusHandler)
	mux.HandleFunc("/api/status", statusHandler)

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
			SameSite: sessionSameSite(r),
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
			SameSite: sessionSameSite(r),
			Secure:   r.TLS != nil,
		})
		w.WriteHeader(http.StatusOK)
	}
	mux.HandleFunc("/logout", logoutHandler)
	mux.HandleFunc("/api/logout", logoutHandler)

	uploadHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorized(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if err := r.ParseMultipartForm(250 << 20); err != nil {
			http.Error(w, "Invalid multipart/form-data", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Missing file in request", http.StatusBadRequest)
			return
		}
		defer file.Close()

		title := strings.TrimSpace(r.FormValue("title"))
		version := strings.TrimSpace(r.FormValue("version"))
		summary := strings.TrimSpace(r.FormValue("summary"))

		sample := make([]byte, 512)
		n, _ := file.Read(sample)
		detected := http.DetectContentType(sample[:n])

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
			"application/pdf":               true,
			"application/zip":               true,
			"application/x-rar-compressed":  true,
			"text/markdown":                 true,
			"text/plain":                    true,
			"application/vnd.ms-powerpoint": true,
			"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
		}

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if !allowedExts[ext] && !allowedMimes[detected] {
			http.Error(w, "File type not allowed", http.StatusBadRequest)
			return
		}

		if !useS3 {
			if err := ensureUploadsDir(); err != nil {
				http.Error(w, "Server error: unable to create uploads directory", http.StatusInternalServerError)
				return
			}
		}

		const maxFileSize = 100 << 20
		reader := io.MultiReader(bytes.NewReader(sample[:n]), file)
		ts := time.Now().UTC().Format("20060102T150405Z")
		safeName := ts + "__" + filepath.Base(header.Filename)
		outPath := filepath.Join(uploadsDir, safeName)

		if useS3 {
			ctx := context.Background()
			if s3Client == nil {
				http.Error(w, "Server error: S3 is not configured", http.StatusInternalServerError)
				return
			}
			_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
				Bucket:      &s3Bucket,
				Key:         &safeName,
				Body:        io.LimitReader(reader, maxFileSize+1),
				ContentType: &detected,
			})
			if err != nil {
				http.Error(w, "Server error: unable to upload to S3", http.StatusInternalServerError)
				return
			}
		} else {
			out, err := os.Create(outPath)
			if err != nil {
				http.Error(w, "Server error: unable to save file", http.StatusInternalServerError)
				return
			}
			defer out.Close()

			written, err := io.Copy(out, io.LimitReader(reader, maxFileSize+1))
			if err != nil {
				http.Error(w, "Server error: unable to write file", http.StatusInternalServerError)
				return
			}
			if written > maxFileSize {
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

	archivesHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
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
	mux.HandleFunc("/deliverables", archivesHandler)
	mux.HandleFunc("/api/deliverables", archivesHandler)

	archiveHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
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

		if useS3 {
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
	if err := http.ListenAndServe(addr, corsMiddleware(mux)); err != nil {
		log.Fatalf("[FATAL] Server failed to start: %v", err)
	}
}
