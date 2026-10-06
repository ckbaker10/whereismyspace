package report

import "testing"

func TestBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{45204476723, "42.1 GB"}, // 42.1 * 1024^3
		{5 * tib, "5.0 TB"},
	}
	for _, c := range cases {
		if got := Bytes(c.in); got != c.want {
			t.Errorf("Bytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"100", 100},
		{"1k", 1024},
		{"1K", 1024},
		{"1KB", 1024},
		{"100M", 100 * mib},
		{"1.5G", int64(1.5 * gib)},
		{"2T", 2 * tib},
		{" 512b ", 512},
	}
	for _, c := range cases {
		got, err := ParseSize(c.in)
		if err != nil {
			t.Errorf("ParseSize(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseSizeErrors(t *testing.T) {
	for _, in := range []string{"", "abc", "12x", "G"} {
		if _, err := ParseSize(in); err == nil {
			t.Errorf("ParseSize(%q) expected error", in)
		}
	}
}
