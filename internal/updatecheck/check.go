// Package updatecheck implements a lightweight, non-blocking background
// check for new scdev releases on GitHub.
//
// Design:
//   - MaybeNotify runs on every CLI invocation.
//   - It reads a cached result from ~/.scdev/update-check.json and, if the
//     cached latest version is newer than the running binary, prints a
//     single-line banner to stderr.
//   - If the cache is older than cacheTTL (or missing), it fires a
//     background goroutine that performs a conditional GET against the
//     GitHub API (using If-None-Match with the stored ETag) with a short
//     timeout. The goroutine is fire-and-forget: the process may exit
//     before it completes, and that's fine. The cache gets refreshed
//     opportunistically over many runs.
//
// Network cost: at most one conditional HTTP request per 24h per machine.
// Most responses are 304 Not Modified with an empty body.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheTTL    = 24 * time.Hour
	httpTimeout = 2 * time.Second
)

// apiURL is a var (not a const) so tests can point it at an httptest server.
var apiURL = "https://api.github.com/repos/ScaleCommerce-DEV/scdev/releases/latest"

type cache struct {
	LastChecked time.Time `json:"last_checked"`
	ETag        string    `json:"etag,omitempty"`
	LatestTag   string    `json:"latest_tag,omitempty"`
}

// MaybeNotify prints a one-line banner to stderr if the cached latest
// release is newer than currentVersion. It also triggers a background
// refresh if the cache is stale. Safe to call on every invocation and
// never blocks. All failures are silent.
func MaybeNotify(currentVersion string) {
	if shouldSkip(currentVersion) {
		return
	}

	path, err := cachePath()
	if err != nil {
		return
	}

	c, _ := loadCache(path) // nil on first run - that's fine

	if c != nil && c.LatestTag != "" && IsNewer(c.LatestTag, currentVersion) {
		fmt.Fprintf(os.Stderr,
			"\x1b[33m→ scdev %s is available (current: %s). Run `scdev self-update`.\x1b[0m\n",
			c.LatestTag, currentVersion)
	}

	if c == nil || time.Since(c.LastChecked) > cacheTTL {
		go refresh(path, c)
	}
}

// shouldSkip returns true for cases where an update check is unwanted:
// dev builds, CI environments, or when the user opts out.
func shouldSkip(v string) bool {
	if v == "" || v == "dev" {
		return true
	}
	if os.Getenv("CI") != "" || os.Getenv("SCDEV_NO_UPDATE_CHECK") != "" {
		return true
	}
	return false
}

// refresh performs a conditional GET and writes the cache. Called in a
// goroutine; the process may exit before this returns.
func refresh(path string, prev *cache) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if prev != nil && prev.ETag != "" {
		req.Header.Set("If-None-Match", prev.ETag)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	next := cache{LastChecked: time.Now()}
	switch resp.StatusCode {
	case http.StatusNotModified:
		if prev == nil {
			return
		}
		next.ETag = prev.ETag
		next.LatestTag = prev.LatestTag
	case http.StatusOK:
		var rel struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
			return
		}
		next.ETag = resp.Header.Get("ETag")
		next.LatestTag = rel.TagName
	default:
		return
	}

	_ = saveCache(path, &next)
}

func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".scdev", "update-check.json"), nil
}

func loadCache(path string) (*cache, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c cache
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func saveCache(path string, c *cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
