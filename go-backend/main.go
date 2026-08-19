// =============================================================================
// go-backend · main.go
//
// Self-Healing System Pipeline — Codebase Ingestion & Analysis Backend
//
// Flow
// ────
//   POST /api/analyze  (multipart/form-data, field "zipfile")
//     1. Receive .zip upload
//     2. Save to a temp file under ./uploads/
//     3. Extract safely into ./uploads/extracted_code_<uuid>/
//     4. Exec: thermodynamic-ast-engine.exe <dir> --output <dir>/report.json
//     5. Read + parse report.json into Go structs
//     6. Stream JSON response → React frontend
//     7. Cleanup temp dir + zip file (background goroutine)
//
// Configuration (environment variables)
// ──────────────────────────────────────
//   ENGINE_PATH          path to thermodynamic-ast-engine.exe  (default: ./thermodynamic-ast-engine.exe)
//   ENGINE_TIMEOUT_SEC   max seconds the engine may run         (default: 300)
//   PORT                 HTTP listen port                        (default: 8080)
//   MAX_UPLOAD_MB        maximum accepted zip size in MiB        (default: 50)
// =============================================================================

package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================================
// § 1  Configuration helpers
// =============================================================================

// envOr returns the value of environment variable `key`, or `fallback` if unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

// =============================================================================
// § 2  Data model — mirrors the Rust engine's JSON output schema
// =============================================================================

// VulnerabilityType is a string enum matching the Rust engine's output.
type VulnerabilityType string

const (
	VulnDeepNesting     VulnerabilityType = "DeepNesting"
	VulnRecursiveCall   VulnerabilityType = "RecursiveCall"
	VulnHotAllocation   VulnerabilityType = "HotAllocation"
	VulnBlockingIO      VulnerabilityType = "BlockingIO"
	VulnCognitiveBranch VulnerabilityType = "CognitiveBranch"
	VulnSyncContention  VulnerabilityType = "SyncContention"
)

// Hotspot represents a single entropy signal within a source file.
// JSON tags match the snake_case keys emitted by the Rust engine.
type Hotspot struct {
	FunctionName      string            `json:"function_name"`
	LineNumber        int               `json:"line_number"`
	SourceSnippet     string            `json:"source_snippet"`
	EntropyScore      float64           `json:"entropy_score"`
	VulnerabilityType VulnerabilityType `json:"vulnerability_type"`
	Description       string            `json:"description"`
}

// FileReport holds the per-file analysis summary.
type FileReport struct {
	FilePath           string    `json:"file_path"`
	Language           string    `json:"language"`
	LinesScanned       int       `json:"lines_scanned"`
	TotalEntropy       float64   `json:"total_entropy"`
	MeanHotspotEntropy float64   `json:"mean_hotspot_entropy"`
	Hotspots           []Hotspot `json:"hotspots"`
}

// ThermodynamicReport is the root document produced by the Rust engine.
type ThermodynamicReport struct {
	EngineVersion    string       `json:"engine_version"`
	GeneratedAt      string       `json:"generated_at"`
	ScannedDirectory string       `json:"scanned_directory"`
	FilesAnalyzed    int          `json:"files_analyzed"`
	TotalHotspots    int          `json:"total_hotspots"`
	GlobalEntropy    float64      `json:"global_entropy"`
	FileReports      []FileReport `json:"file_reports"`
}

// AnalyzeResponse wraps the engine report with backend metadata.
type AnalyzeResponse struct {
	Success bool                 `json:"success"`
	Message string               `json:"message,omitempty"`
	Report  *ThermodynamicReport `json:"report,omitempty"`
}

// ErrorResponse is returned on all 4xx / 5xx paths.
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Detail  string `json:"detail,omitempty"`
}

// =============================================================================
// § 3  Zip extraction — Zip Slip protected
// =============================================================================

// extractZip safely extracts `zipPath` into `destDir`.
//
// Security: every entry's resolved path is checked to confirm it remains
// inside `destDir`; entries that would escape (Zip Slip attack) are rejected.
// See: https://snyk.io/research/zip-slip-vulnerability
func extractZip(zipPath, destDir string) error {
	// Open the zip archive for reading
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	// Resolve destDir to an absolute, cleaned path for prefix checks
	destDir, err = filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("resolve dest dir: %w", err)
	}

	for _, f := range r.File {
		// ── Build the target path ─────────────────────────────────────────────
		// filepath.Join cleans ".." traversals, but we still need an explicit
		// prefix check because a crafted zip could still escape on Windows.
		targetPath := filepath.Join(destDir, filepath.FromSlash(f.Name))

		// Zip Slip guard: ensure targetPath is inside destDir
		if !strings.HasPrefix(
			filepath.Clean(targetPath)+string(os.PathSeparator),
			filepath.Clean(destDir)+string(os.PathSeparator),
		) {
			return fmt.Errorf("zip slip detected: illegal path %q", f.Name)
		}

		if f.FileInfo().IsDir() {
			// Create directory with safe permissions
			if err := os.MkdirAll(targetPath, 0o750); err != nil {
				return fmt.Errorf("create dir %q: %w", targetPath, err)
			}
			continue
		}

		// ── Extract file ──────────────────────────────────────────────────────
		// Ensure parent directory exists (zip may not list directories explicitly)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o750); err != nil {
			return fmt.Errorf("create parent dir for %q: %w", f.Name, err)
		}

		if err := extractSingleFile(f, targetPath); err != nil {
			return fmt.Errorf("extract %q: %w", f.Name, err)
		}
	}

	return nil
}

// extractSingleFile writes one zip entry to disk.
// Files are written with 0640 permissions; no executable bit for safety.
func extractSingleFile(f *zip.File, destPath string) error {
	// Open the compressed stream
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open entry: %w", err)
	}
	defer rc.Close()

	// Create destination file (O_EXCL prevents overwrite attacks)
	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	// Copy with a 256 MiB cap to guard against decompression bombs
	const maxBytes = 256 << 20 // 256 MiB per file
	if _, err := io.Copy(out, io.LimitReader(rc, maxBytes+1)); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// =============================================================================
// § 4  Engine executor
// =============================================================================

// runEngine executes the Rust analysis binary against `codeDir` and expects
// it to write its report to `reportPath`.
//
// Returns the combined stdout+stderr output for logging, or an error if the
// process fails or times out.
func runEngine(ctx context.Context, enginePath, codeDir, reportPath string) (string, error) {
	// Build the command:
	//   thermodynamic-ast-engine.exe <codeDir> --output <reportPath>
	cmd := exec.CommandContext(ctx,
		enginePath,
		codeDir,
		"--output", reportPath,
	)

	// Capture combined output for diagnostic logging
	output, err := cmd.CombinedOutput()
	combinedOutput := string(output)

	if err != nil {
		// Distinguish context timeout from engine failure
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return combinedOutput, fmt.Errorf("engine timed out: %w", ctx.Err())
		}
		// Non-zero exit code from the engine binary
		return combinedOutput, fmt.Errorf("engine exited with error: %w\nOutput:\n%s", err, combinedOutput)
	}

	return combinedOutput, nil
}

// =============================================================================
// § 5  Report reader
// =============================================================================

// readReport reads and parses the JSON report written by the engine.
func readReport(reportPath string) (*ThermodynamicReport, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("read report file: %w", err)
	}

	var report ThermodynamicReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse report JSON: %w", err)
	}

	return &report, nil
}

// =============================================================================
// § 6  Handler
// =============================================================================

// analyzeHandler is the core HTTP handler for POST /api/analyze.
//
// It orchestrates: upload -> extract -> engine -> parse -> respond -> cleanup.
func analyzeHandler(enginePath string, engineTimeout time.Duration, maxUploadBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ── 6a. Receive the uploaded zip ──────────────────────────────────────
		// Limit the reader to avoid OOM on gigantic uploads
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

		// ParseMultipartForm allocates memory for small files, spills larger
		// ones to temp files. 32 MiB in-memory buffer is a reasonable default.
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			log.Printf("[WARN] ParseMultipartForm failed: %v", err)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:  "invalid_multipart",
				Detail: "Could not parse multipart form. Ensure the field is named 'zipfile' and the body is multipart/form-data.",
			})
			return
		}

		// Retrieve the file from the form field named "zipfile"
		fileHeader, err := c.FormFile("zipfile")
		if err != nil {
			log.Printf("[WARN] FormFile('zipfile') error: %v", err)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:  "missing_file",
				Detail: "No file found under form field 'zipfile'.",
			})
			return
		}

		// Basic content-type / extension guard (defence-in-depth)
		filename := fileHeader.Filename
		if !strings.HasSuffix(strings.ToLower(filename), ".zip") {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:  "invalid_file_type",
				Detail: fmt.Sprintf("Expected a .zip file, got: %q", filename),
			})
			return
		}

		// ── 6b. Save the zip to a temp file ──────────────────────────────────
		uploadsDir := "./uploads"
		if err := os.MkdirAll(uploadsDir, 0o750); err != nil {
			log.Printf("[ERROR] Create uploads dir: %v", err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "server_error", Detail: "Could not create uploads directory."})
			return
		}

		// Generate a collision-free temp zip path
		zipDest := filepath.Join(uploadsDir, fmt.Sprintf("upload_%s.zip", uuid.New().String()))

		if err := c.SaveUploadedFile(fileHeader, zipDest); err != nil {
			log.Printf("[ERROR] Save zip to %q: %v", zipDest, err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "server_error", Detail: "Failed to save uploaded file."})
			return
		}

		log.Printf("[INFO] Saved upload: %s (%.2f KiB)", zipDest, float64(fileHeader.Size)/1024)

		// ── 6c. Create a unique extraction directory ──────────────────────────
		extractDir := filepath.Join(uploadsDir,
			fmt.Sprintf("extracted_code_%d_%s", time.Now().UnixMilli(), uuid.New().String()[:8]))

		if err := os.MkdirAll(extractDir, 0o750); err != nil {
			log.Printf("[ERROR] Create extract dir %q: %v", extractDir, err)
			_ = os.Remove(zipDest) // best-effort cleanup before early return
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "server_error", Detail: "Could not create extraction directory."})
			return
		}

		// Register deferred cleanup — runs AFTER the response is sent.
		// Using a goroutine ensures gin has fully flushed the response before
		// we delete files that may still be referenced by the response writer.
		defer func() {
			go func() {
				// Small grace period so the response is fully written
				time.Sleep(500 * time.Millisecond)
				if err := os.RemoveAll(extractDir); err != nil {
					log.Printf("[WARN] Cleanup extract dir %q: %v", extractDir, err)
				} else {
					log.Printf("[INFO] Cleaned up: %s", extractDir)
				}
				if err := os.Remove(zipDest); err != nil {
					log.Printf("[WARN] Cleanup zip %q: %v", zipDest, err)
				} else {
					log.Printf("[INFO] Cleaned up: %s", zipDest)
				}
			}()
		}()

		// ── 6d. Extract zip ───────────────────────────────────────────────────
		log.Printf("[INFO] Extracting %q -> %q", zipDest, extractDir)
		if err := extractZip(zipDest, extractDir); err != nil {
			log.Printf("[ERROR] Zip extraction: %v", err)
			if strings.Contains(err.Error(), "zip slip") {
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error:  "security_violation",
					Detail: "Zip archive contains path traversal entries and was rejected.",
				})
				return
			}
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:  "invalid_zip",
				Detail: fmt.Sprintf("Could not extract archive: %v", err),
			})
			return
		}

		// ── 6e. Run the Rust analysis engine ──────────────────────────────────
		reportPath := filepath.Join(extractDir, "thermodynamic_report.json")

		// Create a context with the configured timeout so the engine
		// cannot run forever (guards against infinite loops in analysed code)
		ctx, cancel := context.WithTimeout(c.Request.Context(), engineTimeout)
		defer cancel()

		log.Printf("[INFO] Launching engine: %q on %q", enginePath, extractDir)
		engineOutput, err := runEngine(ctx, enginePath, extractDir, reportPath)
		if engineOutput != "" {
			log.Printf("[ENGINE] %s", engineOutput)
		}
		if err != nil {
			log.Printf("[ERROR] Engine execution: %v", err)

			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				c.JSON(http.StatusGatewayTimeout, ErrorResponse{
					Error:  "engine_timeout",
					Detail: fmt.Sprintf("Analysis engine did not complete within %.0f seconds.", engineTimeout.Seconds()),
				})
				return
			}

			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:  "engine_failed",
				Detail: fmt.Sprintf("Analysis engine returned an error. Check server logs for details."),
			})
			return
		}

		// ── 6f. Parse the report ──────────────────────────────────────────────
		log.Printf("[INFO] Parsing report: %q", reportPath)
		report, err := readReport(reportPath)
		if err != nil {
			log.Printf("[ERROR] Parse report: %v", err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:  "report_parse_failed",
				Detail: fmt.Sprintf("Engine ran successfully but the report could not be parsed: %v", err),
			})
			return
		}

		// ── 6g. Send response ─────────────────────────────────────────────────
		log.Printf("[INFO] Analysis complete: %d files, %d hotspots, entropy %.2f",
			report.FilesAnalyzed, report.TotalHotspots, report.GlobalEntropy)

		c.JSON(http.StatusOK, AnalyzeResponse{
			Success: true,
			Message: fmt.Sprintf("Analysis complete: %d files scanned, %d hotspots found.", report.FilesAnalyzed, report.TotalHotspots),
			Report:  report,
		})
	}
}

// =============================================================================
// § 7  Server bootstrap
// =============================================================================

func main() {
	// ── Read configuration ────────────────────────────────────────────────────
	enginePath    := envOr("ENGINE_PATH", "./thermodynamic-ast-engine.exe")
	port          := envOr("PORT", "8080")
	timeoutSec    := envOrInt("ENGINE_TIMEOUT_SEC", 300)
	maxUploadMB   := envOrInt("MAX_UPLOAD_MB", 50)

	engineTimeout  := time.Duration(timeoutSec) * time.Second
	maxUploadBytes := int64(maxUploadMB) << 20 // MiB -> bytes

	// Validate that the engine binary exists at startup so we fail fast
	if _, err := os.Stat(enginePath); errors.Is(err, os.ErrNotExist) {
		log.Printf("[WARN] Engine binary not found at %q -- set ENGINE_PATH env var if it is elsewhere.", enginePath)
	} else {
		log.Printf("[INFO] Engine binary confirmed: %s", enginePath)
	}

	// ── Set up Gin ────────────────────────────────────────────────────────────
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()

	// ── Middleware ────────────────────────────────────────────────────────────

	// Structured request logger
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s\n",
			param.TimeStamp.Format(time.RFC3339),
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
		)
	}))

	// Recovery -- converts panics to 500 responses instead of crashing
	r.Use(gin.Recovery())

	// CORS -- allow the React dev server (Vite default: 5173) and common ports.
	// Tighten AllowOrigins in production.
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000", "http://localhost:3001"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// ── Routes ────────────────────────────────────────────────────────────────

	// Health check -- useful for Docker/K8s probes and frontend connectivity tests
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":      "ok",
			"engine_path": enginePath,
			"version":     "1.0.0",
		})
	})

	// Core analysis endpoint
	r.POST("/api/analyze", analyzeHandler(enginePath, engineTimeout, maxUploadBytes))

	// ── Start ─────────────────────────────────────────────────────────────────
	addr := ":" + port
	log.Printf("[INFO] Self-Healing Pipeline backend listening on %s", addr)
	log.Printf("[INFO] Engine: %s | Timeout: %s | Max upload: %d MiB",
		enginePath, engineTimeout, maxUploadMB)

	if err := r.Run(addr); err != nil {
		log.Fatalf("[FATAL] Server failed: %v", err)
	}
}
