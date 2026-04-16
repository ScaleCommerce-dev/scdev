package updatecheck

import (
	"fmt"
	"strconv"
	"strings"
)

// CompareSemver compares two semver strings (with or without "v" prefix).
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareSemver(a, b string) (int, error) {
	partsA, err := ParseSemver(a)
	if err != nil {
		return 0, err
	}
	partsB, err := ParseSemver(b)
	if err != nil {
		return 0, err
	}

	for i := 0; i < 3; i++ {
		if partsA[i] < partsB[i] {
			return -1, nil
		}
		if partsA[i] > partsB[i] {
			return 1, nil
		}
	}
	return 0, nil
}

// ParseSemver parses "major.minor.patch" into [3]int, stripping an optional
// leading "v" and any pre-release suffix.
func ParseSemver(v string) ([3]int, error) {
	v = strings.TrimPrefix(v, "v")
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	var result [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return result, fmt.Errorf("invalid version component %q in %q", parts[i], v)
		}
		result[i] = n
	}
	return result, nil
}

// IsNewer returns true if latest > current. Returns false on parse error
// so callers can safely use it without defensive handling.
func IsNewer(latest, current string) bool {
	cmp, err := CompareSemver(latest, current)
	if err != nil {
		return false
	}
	return cmp > 0
}
