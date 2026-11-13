package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetRemoteRefHash_TagAndSHA(t *testing.T) {
	bareRepo := filepath.Join(t.TempDir(), "test.git")
	run(t, "", "git", "init", "--bare", bareRepo)
	workDir := filepath.Join(t.TempDir(), "work")
	run(t, "", "git", "clone", bareRepo, workDir)
	run(t, workDir, "git", "config", "user.email", "test@test.com")
	run(t, workDir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(workDir, "a.txt"), []byte("a"), 0644)
	run(t, workDir, "git", "add", ".")
	run(t, workDir, "git", "commit", "-m", "c1")
	run(t, workDir, "git", "tag", "-a", "v1", "-m", "v1")
	run(t, workDir, "git", "tag", "light")
	run(t, workDir, "git", "push", "origin", "HEAD", "--tags")
	run(t, workDir, "git", "checkout", "-q", "-b", "deadbeef")
	os.WriteFile(filepath.Join(workDir, "b.txt"), []byte("b"), 0644)
	run(t, workDir, "git", "add", ".")
	run(t, workDir, "git", "commit", "-m", "c2")
	run(t, workDir, "git", "push", "origin", "deadbeef")
	head, _ := GetRemoteRefHash(bareRepo, "")

	t.Run("annotated tag resolves to peeled commit", func(t *testing.T) {
		hash, err := GetRemoteRefHash(bareRepo, "v1")
		if err != nil {
			t.Fatal(err)
		}
		if hash != head {
			t.Errorf("v1 = %s, want commit %s", hash, head)
		}
	})
	t.Run("lightweight tag", func(t *testing.T) {
		hash, err := GetRemoteRefHash(bareRepo, "light")
		if err != nil {
			t.Fatal(err)
		}
		if hash != head {
			t.Errorf("light = %s, want %s", hash, head)
		}
	})
	t.Run("all-hex branch name resolves via remote, not as a sha", func(t *testing.T) {
		hash, err := GetRemoteRefHash(bareRepo, "deadbeef")
		if err != nil {
			t.Fatal(err)
		}
		if hash == "deadbee" || hash == head {
			t.Errorf("deadbeef = %s, want the branch tip", hash)
		}
	})
	t.Run("short sha unknown to remote refs returns itself", func(t *testing.T) {
		hash, err := GetRemoteRefHash(bareRepo, "0123abc")
		if err != nil {
			t.Fatal(err)
		}
		if hash != "0123abc" {
			t.Errorf("hash = %q, want 0123abc", hash)
		}
	})
	t.Run("full commit sha returns itself without network", func(t *testing.T) {
		hash, err := GetRemoteRefHash("/nonexistent/repo.git", "8f14e45fceea167a5a36dedd4bea2543ce848564")
		if err != nil {
			t.Fatal(err)
		}
		if hash != "8f14e45" {
			t.Errorf("hash = %q, want 8f14e45", hash)
		}
	})
}
