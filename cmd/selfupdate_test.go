package cmd

import "testing"

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"equal", "1.2.3", "1.2.3", 0},
		{"a less than b (patch)", "1.2.3", "1.2.4", -1},
		{"a greater than b (patch)", "1.2.4", "1.2.3", 1},
		{"a less than b (minor)", "1.2.0", "1.3.0", -1},
		{"a greater than b (minor)", "1.3.0", "1.2.0", 1},
		{"a less than b (major)", "1.0.0", "2.0.0", -1},
		{"a greater than b (major)", "2.0.0", "1.0.0", 1},
		{"partial version a", "1.0", "1.0.0", 0},
		{"partial version b", "1.0.0", "1.0", 0},
		{"pre-release stripped", "1.0.0-rc1", "1.0.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareSemver(tt.a, tt.b)
			if err != nil {
				t.Fatalf("compareSemver(%q, %q) returned error: %v", tt.a, tt.b, err)
			}
			if got != tt.want {
				t.Errorf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    [3]int
		wantErr bool
	}{
		{"full version", "1.2.3", [3]int{1, 2, 3}, false},
		{"major.minor only", "1.2", [3]int{1, 2, 0}, false},
		{"major only", "1", [3]int{1, 0, 0}, false},
		{"with pre-release", "1.2.3-rc1", [3]int{1, 2, 3}, false},
		{"zeros", "0.0.0", [3]int{0, 0, 0}, false},
		{"invalid", "abc", [3]int{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSemver(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseSemver(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSemver(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseSemver(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSelfUpdateBinaryName(t *testing.T) {
	name := selfUpdateBinaryName()
	if name == "" {
		t.Error("selfUpdateBinaryName() returned empty string")
	}
	if name[:6] != "scdev-" {
		t.Errorf("selfUpdateBinaryName() = %q, want prefix 'scdev-'", name)
	}
}
