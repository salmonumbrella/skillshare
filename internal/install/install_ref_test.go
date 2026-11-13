package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsCommitSHA(t *testing.T) {
	cases := map[string]bool{
		"8f14e45fceea167a5a36dedd4bea2543ce848564": true,
		"8f14e45":   true,
		"main":      false,
		"v1.2.3":    false,
		"8f14e4":    false, // too short
		"deadbeefg": false, // non-hex
		"":          false,
	}
	for ref, want := range cases {
		if got := IsCommitSHA(ref); got != want {
			t.Errorf("IsCommitSHA(%q) = %v, want %v", ref, got, want)
		}
	}
}

// setupPinRemote creates a bare file:// remote with two commits on main, an
// annotated tag v1 on the first commit, and a feature branch with its own commit.
// Returns the URL, the first main commit SHA, and the feature commit SHA.
func setupPinRemote(t *testing.T) (url, firstSHA, featureSHA string) {
	t.Helper()
	base := t.TempDir()
	bare := filepath.Join(base, "remote.git")
	runGit(t, "", "init", "--bare", bare)
	seed := filepath.Join(base, "seed")
	runGit(t, "", "clone", bare, seed)
	runGit(t, seed, "config", "user.email", "test@test.com")
	runGit(t, seed, "config", "user.name", "test")
	os.WriteFile(filepath.Join(seed, "SKILL.md"), []byte("# v1"), 0644)
	runGit(t, seed, "add", ".")
	runGit(t, seed, "commit", "-m", "c1")
	runGit(t, seed, "tag", "-a", "v1", "-m", "v1")
	os.WriteFile(filepath.Join(seed, "SKILL.md"), []byte("# v2"), 0644)
	runGit(t, seed, "add", ".")
	runGit(t, seed, "commit", "-m", "c2")
	runGit(t, seed, "push", "origin", "HEAD:main", "--tags")
	runGit(t, bare, "symbolic-ref", "HEAD", "refs/heads/main")
	runGit(t, seed, "checkout", "-q", "-b", "feature")
	os.WriteFile(filepath.Join(seed, "SKILL.md"), []byte("# feature"), 0644)
	runGit(t, seed, "add", ".")
	runGit(t, seed, "commit", "-m", "feature")
	runGit(t, seed, "push", "origin", "feature")

	revParse := func(rev string) string {
		out, err := exec.Command("git", "-C", seed, "rev-parse", rev).Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	return "file://" + bare, revParse("origin/main~1"), revParse("feature")
}

func TestCloneRepo_PinnedRef(t *testing.T) {
	url, sha, featureSHA := setupPinRemote(t)
	for name, tc := range map[string]struct{ ref, want string }{
		"full sha":                     {sha, sha},
		"short sha":                    {sha[:7], sha},
		"tag":                          {"v1", sha},
		"short sha off default branch": {featureSHA[:7], featureSHA},
	} {
		t.Run(name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "repo")
			if err := cloneRepo(url, dest, tc.ref, true, nil); err != nil {
				t.Fatalf("cloneRepo(%q): %v", tc.ref, err)
			}
			got, err := getGitFullHash(dest)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("HEAD = %s, want %s", got, tc.want)
			}
		})
	}
}
