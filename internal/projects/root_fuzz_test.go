package projects

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/utils"
)

func FuzzProjectRootIdentity(f *testing.F) {
	for _, seed := range []string{".", "app", "../app/", "~/work/../app///", "~", "/", "./a//b/..", "with spaces"} {
		f.Add(seed)
	}
	cwd := filepath.Join(f.TempDir(), "home", "work")
	home := filepath.Dir(cwd)
	f.Fuzz(func(t *testing.T, arg string) {
		if arg == "" || strings.ContainsRune(arg, 0) ||
			(strings.HasPrefix(arg, "~") && len(arg) > 1 && !os.IsPathSeparator(arg[1])) ||
			(runtime.GOOS == "windows" && filepath.VolumeName(arg) != "" && !filepath.IsAbs(arg)) {
			return
		}
		identity, err := Identity(arg, cwd, home)
		if err != nil {
			t.Fatalf("valid root %q rejected: %v", arg, err)
		}
		again, err := Identity(identity, cwd, home)
		if err != nil || !utils.PathsEqual(identity, again) {
			t.Fatalf("Identity is not idempotent: %q => %q => %q, %v", arg, identity, again, err)
		}
		if strings.HasPrefix(arg, "~") {
			absolute, err := Identity(filepath.Join(home, arg[1:]), cwd, home)
			if err != nil || !utils.PathsEqual(identity, absolute) {
				t.Fatalf("tilde and absolute spellings disagree for %q: %q, %q, %v", arg, identity, absolute, err)
			}
			return
		}
		// Relative paths give the same identity under the explicit home and ~.
		if filepath.IsAbs(arg) || filepath.VolumeName(arg) != "" {
			return
		}
		tilde, err := Identity("~/"+arg, cwd, home)
		if err != nil {
			t.Fatalf("valid relative root %q rejected under home: %v", arg, err)
		}
		absolute, err := Identity(filepath.Join(home, arg), cwd, home)
		if err != nil || !utils.PathsEqual(tilde, absolute) {
			t.Fatalf("home spellings disagree for %q: %q, %q, %v", arg, tilde, absolute, err)
		}
	})
}
