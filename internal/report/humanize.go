// Package report renders scan results as a top-N list, a depth-limited tree, or
// JSON, plus a shared summary header.
package report

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	kib = 1024
	mib = 1024 * kib
	gib = 1024 * mib
	tib = 1024 * gib
	pib = 1024 * tib
)

// Bytes formats a byte count as a human-readable IEC size, e.g. "42.1 GB".
func Bytes(b int64) string {
	switch {
	case b >= pib:
		return fmt.Sprintf("%.1f PB", float64(b)/pib)
	case b >= tib:
		return fmt.Sprintf("%.1f TB", float64(b)/tib)
	case b >= gib:
		return fmt.Sprintf("%.1f GB", float64(b)/gib)
	case b >= mib:
		return fmt.Sprintf("%.1f MB", float64(b)/mib)
	case b >= kib:
		return fmt.Sprintf("%.1f KB", float64(b)/kib)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// ParseSize parses a human size like "100M", "1.5G", "500k" or a bare byte
// count into a number of bytes. It accepts optional K/M/G/T/P suffixes (with an
// optional trailing "B"), case-insensitive, using 1024-based units.
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	orig := s
	s = strings.TrimSuffix(strings.ToUpper(s), "B")
	if s == "" {
		return 0, fmt.Errorf("invalid size %q", orig)
	}

	mult := int64(1)
	switch s[len(s)-1] {
	case 'K':
		mult, s = kib, s[:len(s)-1]
	case 'M':
		mult, s = mib, s[:len(s)-1]
	case 'G':
		mult, s = gib, s[:len(s)-1]
	case 'T':
		mult, s = tib, s[:len(s)-1]
	case 'P':
		mult, s = pib, s[:len(s)-1]
	}
	s = strings.TrimSpace(s)

	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v * mult, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", orig)
	}
	return int64(f * float64(mult)), nil
}
