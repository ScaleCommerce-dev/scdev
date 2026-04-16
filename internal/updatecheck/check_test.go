package updatecheck

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldSkip(t *testing.T) {
	// Isolate env so we don't inherit CI=true from the test runner.
	t.Setenv("CI", "")
	t.Setenv("SCDEV_NO_UPDATE_CHECK", "")

	tests := []struct {
		name    string
		version string
		envKey  string
		envVal  string
		want    bool
	}{
		{"empty version", "", "", "", true},
		{"dev build", "dev", "", "", true},
		{"normal version", "v0.5.6", "", "", false},
		{"CI set", "v0.5.6", "CI", "true", true},
		{"opt-out set", "v0.5.6", "SCDEV_NO_UPDATE_CHECK", "1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envKey != "" {
				t.Setenv(tt.envKey, tt.envVal)
			}
			if got := shouldSkip(tt.version); got != tt.want {
				t.Errorf("shouldSkip(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestCacheLoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	// Missing file -> error, nil cache
	if c, err := loadCache(path); err == nil || c != nil {
		t.Fatalf("expected error for missing cache, got cache=%v err=%v", c, err)
	}

	want := &cache{
		LastChecked: time.Now().UTC().Truncate(time.Second),
		ETag:        `W/"abc123"`,
		LatestTag:   "v0.5.7",
	}
	if err := saveCache(path, want); err != nil {
		t.Fatalf("saveCache: %v", err)
	}

	got, err := loadCache(path)
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	if got.ETag != want.ETag || got.LatestTag != want.LatestTag {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}
	if !got.LastChecked.Equal(want.LastChecked) {
		t.Errorf("LastChecked mismatch: got %v, want %v", got.LastChecked, want.LastChecked)
	}
}

func TestSaveCacheCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deeper", "cache.json")
	if err := saveCache(path, &cache{LatestTag: "v1.0.0"}); err != nil {
		t.Fatalf("saveCache with missing parents: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist, stat err: %v", err)
	}
}

func TestRefreshHandles200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `W/"fresh"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	defer srv.Close()

	prev := apiURL
	apiURL = srv.URL
	defer func() { apiURL = prev }()

	path := filepath.Join(t.TempDir(), "cache.json")
	refresh(path, nil)

	got, err := loadCache(path)
	if err != nil {
		t.Fatalf("cache not written: %v", err)
	}
	if got.LatestTag != "v9.9.9" {
		t.Errorf("LatestTag = %q, want v9.9.9", got.LatestTag)
	}
	if got.ETag != `W/"fresh"` {
		t.Errorf("ETag = %q, want fresh", got.ETag)
	}
}

func TestRefreshHandles304(t *testing.T) {
	var sawIfNoneMatch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawIfNoneMatch = r.Header.Get("If-None-Match")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	prev := apiURL
	apiURL = srv.URL
	defer func() { apiURL = prev }()

	path := filepath.Join(t.TempDir(), "cache.json")
	old := &cache{
		LastChecked: time.Now().Add(-48 * time.Hour),
		ETag:        `W/"cached"`,
		LatestTag:   "v0.5.7",
	}
	refresh(path, old)

	if sawIfNoneMatch != `W/"cached"` {
		t.Errorf("If-None-Match header = %q, want cached", sawIfNoneMatch)
	}
	got, err := loadCache(path)
	if err != nil {
		t.Fatalf("cache not written: %v", err)
	}
	if got.LatestTag != "v0.5.7" || got.ETag != `W/"cached"` {
		t.Errorf("304 should keep prev values, got %+v", got)
	}
	if time.Since(got.LastChecked) > time.Minute {
		t.Errorf("LastChecked not refreshed: %v", got.LastChecked)
	}
}

func TestRefreshDoesNotWriteOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	prev := apiURL
	apiURL = srv.URL
	defer func() { apiURL = prev }()

	path := filepath.Join(t.TempDir(), "cache.json")
	refresh(path, nil)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected no cache file on 500, got err=%v", err)
	}
}
