package cmd

import "testing"

func TestSelfUpdateBinaryName(t *testing.T) {
	name := selfUpdateBinaryName()
	if name == "" {
		t.Error("selfUpdateBinaryName() returned empty string")
	}
	if name[:6] != "scdev-" {
		t.Errorf("selfUpdateBinaryName() = %q, want prefix 'scdev-'", name)
	}
}
