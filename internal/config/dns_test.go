package config

import (
	"testing"
)

func TestContainsLoopback(t *testing.T) {
	tests := []struct {
		name     string
		ips      []string
		expected bool
	}{
		{
			name:     "contains 127.0.0.1",
			ips:      []string{"127.0.0.1"},
			expected: true,
		},
		{
			name:     "contains 127.0.0.1 among others",
			ips:      []string{"8.8.8.8", "127.0.0.1", "1.1.1.1"},
			expected: true,
		},
		{
			name:     "contains IPv6 loopback",
			ips:      []string{"::1"},
			expected: true,
		},
		{
			name:     "no loopback",
			ips:      []string{"8.8.8.8", "1.1.1.1"},
			expected: false,
		},
		{
			name:     "empty list",
			ips:      []string{},
			expected: false,
		},
		{
			name:     "127.0.0.2 is also loopback",
			ips:      []string{"127.0.0.2"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsLoopback(tt.ips)
			if result != tt.expected {
				t.Errorf("containsLoopback(%v) = %v, want %v", tt.ips, result, tt.expected)
			}
		})
	}
}

func TestVerifyDomainDNS_DefaultDomain(t *testing.T) {
	// Verify the wildcard under DefaultDomain resolves to 127.0.0.1.
	// We test a subdomain rather than the apex because zdev only uses
	// `*.<domain>` (project URLs are always `<project>.<domain>`); the
	// apex itself need not have an A record.
	probe := "zdev-dns-probe." + DefaultDomain
	result, err := VerifyDomainDNS(probe)

	if err != nil {
		if result != nil && result.ResolvesTo127 {
			t.Skipf("Domain %s is correctly configured but system DNS doesn't resolve it: %v", probe, err)
		}
		t.Errorf("Wildcard under %s should resolve to 127.0.0.1: %v", DefaultDomain, err)
	}
}

func TestVerifyDomainDNS_InvalidDomain(t *testing.T) {
	// Test with a domain that doesn't resolve to 127.0.0.1
	_, err := VerifyDomainDNS("google.com")
	if err == nil {
		t.Error("Expected error for domain that doesn't resolve to 127.0.0.1")
	}
}
