package utils

import (
	"io/fs"
	"testing"
)

func TestIsLinkMode(t *testing.T) {
	prev := isJunction
	t.Cleanup(func() { isJunction = prev })
	isJunction = func(path string) bool { return path == "junction" }

	cases := []struct {
		path string
		mode fs.FileMode
		want bool
	}{
		{"link", fs.ModeSymlink, true},
		{"junction", fs.ModeIrregular, true}, // Go 1.23+ on Windows
		{"placeholder", fs.ModeIrregular, false},
		{"junction", 0, false}, // a regular file is never probed
		{"dir", fs.ModeDir, false},
	}
	for _, c := range cases {
		if got := IsLinkMode(c.path, c.mode); got != c.want {
			t.Errorf("IsLinkMode(%q, %v) = %v, want %v", c.path, c.mode, got, c.want)
		}
	}
}
