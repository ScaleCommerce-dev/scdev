package project

import (
	"testing"
)

func TestIsPortAvailable(t *testing.T) {
	// Test that a random high port is available
	if !isPortAvailable("127.0.0.1:0") {
		t.Error("expected random port to be available")
	}
}
