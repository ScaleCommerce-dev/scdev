package config

import (
	"strings"
	"testing"
)

// HTML escaping is handled by stdlib html.EscapeString; this package just
// wires it into generateProjectsSection. The XSS-escaping behavior is
// verified end-to-end in TestGenerateProjectsSection's "HTML escaping"
// subtest, which is the contract that actually matters.

func TestGenerateProjectsSection(t *testing.T) {
	t.Run("empty projects list", func(t *testing.T) {
		result := generateProjectsSection(nil, "https")

		if !strings.Contains(result, "No projects configured yet") {
			t.Error("expected 'No projects configured yet' message for empty list")
		}
		if !strings.Contains(result, "section-label") {
			t.Error("expected section-label class")
		}
	})

	t.Run("single running project", func(t *testing.T) {
		projects := []ProjectInfo{
			{
				Name:    "myshop",
				Domain:  "myshop.scalecommerce.site",
				Path:    "/home/user/projects/myshop",
				Running: true,
			},
		}

		result := generateProjectsSection(projects, "https")

		if !strings.Contains(result, `class="project-row running"`) {
			t.Error("expected project-row with running class")
		}
		if !strings.Contains(result, `class="project-status running"`) {
			t.Error("expected project-status with running class")
		}
		if !strings.Contains(result, ">running<") {
			t.Error("expected running status text")
		}
		if !strings.Contains(result, "https://myshop.scalecommerce.site") {
			t.Error("expected project URL with https protocol")
		}
		if !strings.Contains(result, "myshop") {
			t.Error("expected project name in output")
		}
		if !strings.Contains(result, "/home/user/projects/myshop") {
			t.Error("expected project path in output")
		}
		if !strings.Contains(result, "projects-list") {
			t.Error("expected projects-list class")
		}
	})

	t.Run("multiple projects mixed states", func(t *testing.T) {
		projects := []ProjectInfo{
			{
				Name:    "shop-a",
				Domain:  "shop-a.scalecommerce.site",
				Path:    "/projects/shop-a",
				Running: true,
			},
			{
				Name:    "shop-b",
				Domain:  "shop-b.scalecommerce.site",
				Path:    "/projects/shop-b",
				Running: false,
			},
			{
				Name:    "shop-c",
				Domain:  "shop-c.scalecommerce.site",
				Path:    "/projects/shop-c",
				Running: true,
			},
		}

		result := generateProjectsSection(projects, "http")

		// Count running and stopped rows
		runningCount := strings.Count(result, `class="project-row running"`)
		stoppedCount := strings.Count(result, `class="project-row stopped"`)

		if runningCount != 2 {
			t.Errorf("expected 2 running project rows, got %d", runningCount)
		}
		if stoppedCount != 1 {
			t.Errorf("expected 1 stopped project row, got %d", stoppedCount)
		}

		// Verify HTTP protocol is used
		if !strings.Contains(result, "http://shop-a.scalecommerce.site") {
			t.Error("expected http protocol in URL")
		}
		if strings.Contains(result, "https://") {
			t.Error("should not contain https when protocol is http")
		}

		// All three projects should be present
		if !strings.Contains(result, "shop-a") {
			t.Error("expected shop-a in output")
		}
		if !strings.Contains(result, "shop-b") {
			t.Error("expected shop-b in output")
		}
		if !strings.Contains(result, "shop-c") {
			t.Error("expected shop-c in output")
		}
	})

	t.Run("HTML escaping of project names", func(t *testing.T) {
		projects := []ProjectInfo{
			{
				Name:    "<script>alert('xss')</script>",
				Domain:  "safe.scalecommerce.site",
				Path:    "/projects/<evil>",
				Running: true,
			},
		}

		result := generateProjectsSection(projects, "https")

		// The raw HTML tags should NOT appear
		if strings.Contains(result, "<script>") {
			t.Error("raw <script> tag should be escaped")
		}

		// Escaped versions should appear
		if !strings.Contains(result, "&lt;script&gt;") {
			t.Error("expected escaped <script> tag in project name")
		}
		if !strings.Contains(result, "&lt;evil&gt;") {
			t.Error("expected escaped <evil> in project path")
		}
	})
}
