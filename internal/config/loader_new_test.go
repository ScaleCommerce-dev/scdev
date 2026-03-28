package config

import (
	"strings"
	"testing"
)

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ampersand",
			input:    "foo & bar",
			expected: "foo &amp; bar",
		},
		{
			name:     "less than",
			input:    "a < b",
			expected: "a &lt; b",
		},
		{
			name:     "greater than",
			input:    "a > b",
			expected: "a &gt; b",
		},
		{
			name:     "double quote",
			input:    `say "hello"`,
			expected: "say &quot;hello&quot;",
		},
		{
			name:     "single quote",
			input:    "it's",
			expected: "it&#39;s",
		},
		{
			name:     "no special characters",
			input:    "plain text 123",
			expected: "plain text 123",
		},
		{
			name:     "all special characters",
			input:    `&<>"'`,
			expected: "&amp;&lt;&gt;&quot;&#39;",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeHTML(tt.input)
			if result != tt.expected {
				t.Errorf("escapeHTML(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

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
