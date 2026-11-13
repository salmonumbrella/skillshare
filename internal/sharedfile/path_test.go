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
		{`.\rules`, filepath.Join(home, "team", "rules"), true},
		{`extras\nested/rules`, filepath.Join(home, "team", "extras", "nested", "rules"), true},
		{`..\rules`, filepath.Join(home, "rules"), true},
		{`~\rules\nested`, filepath.Join(home, "rules", "nested"), true},
		{"~", home, true},
		{filepath.Join(home, "absolute"), filepath.Join(home, "absolute"), false},
		{`C:\machine\rules`, filepath.Clean(`C:\machine\rules`), false},
		{`\\server\share\rules`, filepath.Clean(`\\server\share\rules`), false},
	} {
		if got := ResolvePath(filepath.Join(home, "team"), tc.in); got != tc.want {
			t.Errorf("resolve %q=%q want=%q", tc.in, got, tc.want)
		}
		if got := IsPortablePath(tc.in); got != tc.portable {
			t.Errorf("portable %q=%v", tc.in, got)
		}
	}
}
