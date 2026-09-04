package cmd

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.4.1", "1.4.1", 0},
		{"v1.10.0", "v1.9.0", 1},
		{"v1.4.0", "v1.4.1", -1},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.0.0", "v1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"v1.4.1", "v1.4.1", 0},
		{"v1.9.9", "v1.10.0", -1},
	}
	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
