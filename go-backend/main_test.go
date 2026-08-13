// go-backend · main_test.go
//
// Unit tests for the utility functions — no HTTP server or engine required.
// Run with: go test ./... -v

package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// ── extractZip tests ──────────────────────────────────────────────────────────

// makeTestZip creates a small in-memory zip and writes it to `path`.
func makeTestZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	for name, content := range entries {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatalf("create entry %q: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write entry %q: %v", name, err)
		}
	}
}

// TestExtractZip_HappyPath verifies that a well-formed zip is extracted correctly.
func TestExtractZip_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "test.zip")

	makeTestZip(t, zipPath, map[string]string{
		"hello.py":      "print('hello')\n",
		"sub/world.go":  "package main\n",
	})

	destDir := filepath.Join(tmp, "out")
	if err := extractZip(zipPath, destDir); err != nil {
		t.Fatalf("extractZip failed: %v", err)
	}

	// Verify files exist with expected content
	checkFile := func(rel, want string) {
		t.Helper()
		got, err := os.ReadFile(filepath.Join(destDir, rel))
		if err != nil {
			t.Errorf("read %q: %v", rel, err)
			return
		}
		if string(got) != want {
			t.Errorf("%q: got %q, want %q", rel, got, want)
		}
	}

	checkFile("hello.py", "print('hello')\n")
	checkFile("sub/world.go", "package main\n")
}

// TestExtractZip_ZipSlip ensures a path-traversal zip is rejected.
func TestExtractZip_ZipSlip(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "evil.zip")

	// Manually craft a zip with a path traversal entry
	f, _ := os.Create(zipPath)
	w := zip.NewWriter(f)
	// "../escape.txt" would land outside the dest dir
	entry, _ := w.Create("../../escape.txt")
	_, _ = entry.Write([]byte("pwned"))
	w.Close()
	f.Close()

	destDir := filepath.Join(tmp, "out")
	err := extractZip(zipPath, destDir)
	if err == nil {
		t.Fatal("expected error for zip slip, got nil")
	}
	t.Logf("correctly rejected: %v", err)
}

// TestExtractZip_CorruptZip verifies that a corrupted zip returns an error.
func TestExtractZip_CorruptZip(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "corrupt.zip")
	// Write garbage bytes
	if err := os.WriteFile(zipPath, []byte("this is not a zip file"), 0o640); err != nil {
		t.Fatal(err)
	}

	err := extractZip(zipPath, filepath.Join(tmp, "out"))
	if err == nil {
		t.Fatal("expected error for corrupt zip, got nil")
	}
	t.Logf("correctly rejected: %v", err)
}

// ── readReport tests ──────────────────────────────────────────────────────────

// TestReadReport_Valid verifies that a well-formed JSON report is parsed.
func TestReadReport_Valid(t *testing.T) {
	tmp := t.TempDir()
	reportPath := filepath.Join(tmp, "report.json")

	json := `{
		"engine_version": "0.1.0",
		"generated_at": "2026-08-13T00:00:00Z",
		"scanned_directory": "/tmp/code",
		"files_analyzed": 2,
		"total_hotspots": 5,
		"global_entropy": 123.45,
		"file_reports": []
	}`

	if err := os.WriteFile(reportPath, []byte(json), 0o640); err != nil {
		t.Fatal(err)
	}

	report, err := readReport(reportPath)
	if err != nil {
		t.Fatalf("readReport: %v", err)
	}

	if report.FilesAnalyzed != 2 {
		t.Errorf("FilesAnalyzed: got %d, want 2", report.FilesAnalyzed)
	}
	if report.GlobalEntropy != 123.45 {
		t.Errorf("GlobalEntropy: got %f, want 123.45", report.GlobalEntropy)
	}
	if report.EngineVersion != "0.1.0" {
		t.Errorf("EngineVersion: got %q, want 0.1.0", report.EngineVersion)
	}
}

// TestReadReport_MissingFile verifies error on absent report.
func TestReadReport_MissingFile(t *testing.T) {
	_, err := readReport("/nonexistent/path/report.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	t.Logf("correctly rejected: %v", err)
}

// TestReadReport_InvalidJSON verifies error on malformed JSON.
func TestReadReport_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(path, []byte("{broken json"), 0o640); err != nil {
		t.Fatal(err)
	}

	_, err := readReport(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	t.Logf("correctly rejected: %v", err)
}

// ── envOr / envOrInt tests ────────────────────────────────────────────────────

func TestEnvOr(t *testing.T) {
	t.Setenv("TEST_KEY", "hello")
	if got := envOr("TEST_KEY", "fallback"); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	if got := envOr("UNSET_KEY_XYZ", "fallback"); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestEnvOrInt(t *testing.T) {
	t.Setenv("TEST_INT", "42")
	if got := envOrInt("TEST_INT", 0); got != 42 {
		t.Errorf("got %d, want 42", got)
	}
	if got := envOrInt("UNSET_INT_XYZ", 99); got != 99 {
		t.Errorf("got %d, want 99", got)
	}
}
