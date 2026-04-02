package create

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// DefaultGitHubOrg is the default GitHub organization for template shorthand names
	DefaultGitHubOrg = "ScaleCommerce-DEV"

	// TemplateRepoPrefix is prepended to bare template names for GitHub resolution
	TemplateRepoPrefix = "scdev-template-"
)

// TemplateSource represents where a template comes from
type TemplateSource struct {
	Type  string // "local" or "github"
	Path  string // local: absolute path
	Owner string // github: org/user
	Repo  string // github: repository name
	Ref   string // github: branch or tag (empty = repo default)
}

// nameRegex validates DNS-safe project names:
// - lowercase alphanumeric + hyphens
// - no leading/trailing hyphens
// - 1-63 characters
var nameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// githubIdentRegex validates GitHub owner and repo names
var githubIdentRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ValidateName checks that a project name is DNS-safe
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("project name %q is invalid: must contain only lowercase letters, numbers, and hyphens (no leading/trailing hyphens, max 63 chars)", name)
	}
	return nil
}

// ResolveTemplate parses a template argument into a TemplateSource
//
// Resolution rules:
//   - Starts with /, ./, ../, ~ -> local directory
//   - Contains / (e.g. myorg/myrepo) -> GitHub owner/repo
//   - Bare name (e.g. express) -> GitHub ScaleCommerce-DEV/scdev-template-<name>
func ResolveTemplate(template, branch, tag string) (*TemplateSource, error) {
	if branch != "" && tag != "" {
		return nil, fmt.Errorf("--branch and --tag are mutually exclusive")
	}

	ref := branch
	if tag != "" {
		ref = tag
	}

	// Local path detection
	if isLocalPath(template) {
		if ref != "" {
			return nil, fmt.Errorf("--branch and --tag can only be used with GitHub templates")
		}

		path := template
		// Expand ~/ to home directory (not ~user/ syntax)
		if path == "~" || strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to expand ~: %w", err)
			}
			path = filepath.Join(home, path[2:])
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path: %w", err)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("template directory not found: %s", absPath)
			}
			return nil, fmt.Errorf("failed to access template directory: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("template path is not a directory: %s", absPath)
		}

		return &TemplateSource{
			Type: "local",
			Path: absPath,
		}, nil
	}

	// GitHub: contains / -> owner/repo
	if strings.Contains(template, "/") {
		parts := strings.SplitN(template, "/", 2)
		if !githubIdentRegex.MatchString(parts[0]) || !githubIdentRegex.MatchString(parts[1]) {
			return nil, fmt.Errorf("invalid GitHub repository: %s (owner and repo must contain only alphanumeric characters, dots, hyphens, and underscores)", template)
		}
		return &TemplateSource{
			Type:  "github",
			Owner: parts[0],
			Repo:  parts[1],
			Ref:   ref,
		}, nil
	}

	// Bare name -> default org with prefix
	if !githubIdentRegex.MatchString(template) {
		return nil, fmt.Errorf("invalid template name: %s (must contain only alphanumeric characters, dots, hyphens, and underscores)", template)
	}
	return &TemplateSource{
		Type:  "github",
		Owner: DefaultGitHubOrg,
		Repo:  TemplateRepoPrefix + template,
		Ref:   ref,
	}, nil
}

// isLocalPath returns true if the template string looks like a local filesystem path
func isLocalPath(template string) bool {
	return strings.HasPrefix(template, "/") ||
		strings.HasPrefix(template, "./") ||
		strings.HasPrefix(template, "../") ||
		strings.HasPrefix(template, "~/") ||
		template == "." ||
		template == ".." ||
		template == "~"
}

// CopyLocal copies a local template directory to the target, excluding .git/
func CopyLocal(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from source
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to compute relative path: %w", err)
		}

		// Skip .git directory
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		targetPath := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// Copy file
		return copyFile(path, targetPath)
	})
}

// copyFile copies a single file preserving permissions
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create target file %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy %s: %w", src, err)
	}

	return nil
}

// DownloadGitHub downloads and extracts a GitHub repo tarball into the target directory
func DownloadGitHub(ctx context.Context, source *TemplateSource, dst string) error {
	// Build URL
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball", source.Owner, source.Repo)
	if source.Ref != "" {
		url += "/" + source.Ref
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download template: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// OK
	case http.StatusNotFound:
		return fmt.Errorf("template repository %s/%s not found on GitHub", source.Owner, source.Repo)
	case http.StatusForbidden:
		return fmt.Errorf("GitHub API rate limit exceeded, try again later")
	default:
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	return extractTarGz(resp.Body, dst)
}

// extractTarGz extracts a gzipped tar archive, stripping the root directory
func extractTarGz(r io.Reader, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to decompress: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	// The first entry is the root directory (e.g. "owner-repo-sha/")
	// We need to detect and strip it
	var rootPrefix string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Skip pax headers (GitHub tarballs include these before the actual content)
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			continue
		}

		// Detect root prefix from the first real entry
		if rootPrefix == "" {
			rootPrefix = strings.SplitN(header.Name, "/", 2)[0] + "/"
		}

		// Strip root prefix
		name := strings.TrimPrefix(header.Name, rootPrefix)
		if name == "" {
			continue // Skip the root directory itself
		}

		targetPath := filepath.Join(dst, name)

		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(dst)) {
			return fmt.Errorf("tar entry %q attempts path traversal", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", name, err)
			}

		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Strip special bits (setuid/setgid), keep rwx only
			mode := os.FileMode(header.Mode) & 0777
			if mode == 0 {
				mode = 0644
			}

			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", name, err)
			}

			// Limit file size to 1GB to prevent tar bombs
			const maxFileSize = 1 << 30
			if _, err := io.Copy(f, io.LimitReader(tr, maxFileSize)); err != nil {
				f.Close()
				return fmt.Errorf("failed to write file %s: %w", name, err)
			}
			f.Close()

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			// Validate symlink target stays within dst to prevent path traversal
			linkTarget := header.Linkname
			if filepath.IsAbs(linkTarget) {
				return fmt.Errorf("tar entry %q contains absolute symlink target %q", header.Name, linkTarget)
			}
			resolvedLink := filepath.Join(filepath.Dir(targetPath), linkTarget)
			if !strings.HasPrefix(filepath.Clean(resolvedLink), filepath.Clean(dst)) {
				return fmt.Errorf("tar entry %q symlink target %q escapes destination", header.Name, linkTarget)
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", name, err)
			}
		}
	}

	return nil
}

// DisplayName returns a human-readable name for the template source
func (s *TemplateSource) DisplayName() string {
	if s.Type == "local" {
		return s.Path
	}
	name := s.Owner + "/" + s.Repo
	if s.Ref != "" {
		name += "@" + s.Ref
	}
	return name
}
