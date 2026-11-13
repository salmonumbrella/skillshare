package sharedfile

import (
	"path/filepath"
	"testing"
)

func TestPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, tc := range []struct {
		in, want string
		portable bool
	}{
		{"", "", true}, {"~/x", filepath.Join(home, "x"), true},
		{"extras/rules", filepath.Join(home, "team", "extras", "rules"), true},
		{filepath.Join(home, "absolute"), filepath.Join(home, "absolute"), false},
		{"C:\\machine\\rules", "C:\\machine\\rules", false},
	} {
		if got := ResolvePath(filepath.Join(home, "team"), tc.in); got != tc.want {
			t.Errorf("resolve %q=%q want=%q", tc.in, got, tc.want)
		}
		if got := IsPortablePath(tc.in); got != tc.portable {
			t.Errorf("portable %q=%v", tc.in, got)
		}
	}
}
