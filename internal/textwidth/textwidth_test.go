package textwidth

import "testing"

func TestVisible(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "abc", 3},
		{"ansi stripped", "\x1b[31mred\x1b[0m", 3},
		{"wide runes count double", "你好", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Visible(tc.in); got != tc.want {
				t.Errorf("Visible(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
