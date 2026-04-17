package updatecheck

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// ChecksumsAssetName is the filename of the sha256 checksums file attached
// to every scdev release. Keep in sync with .github/workflows/release.yml.
const ChecksumsAssetName = "checksums.txt"

// ParseChecksums parses the output of `sha256sum` into {filename -> sha256}.
// Lines not matching the "<hex>  <filename>" format are skipped.
func ParseChecksums(r io.Reader) map[string]string {
	sums := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// sha256sum format: "<hex>  <filename>" (two spaces) or "<hex> *<filename>" (binary mode)
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		hash := fields[0]
		name := strings.TrimPrefix(fields[1], "*")
		if len(hash) != 64 {
			continue
		}
		sums[name] = hash
	}
	return sums
}

// VerifyFile computes the sha256 of the file at path and compares to expected.
// Returns nil on match, a descriptive error on mismatch or read failure.
func VerifyFile(path, expectedHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("read for checksum: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expectedHex {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHex, actual)
	}
	return nil
}
