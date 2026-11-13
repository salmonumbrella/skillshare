package git

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestUnpushedBlobsKeepsEarlierBytesAfterRemoval(t *testing.T) {
	for _, path := range []string{"config.shared.yaml", "team.yaml", "nested/team file.yaml"} {
		t.Run(path, func(t *testing.T) {
			dir := t.TempDir()
			gitExec(t, dir, "init")
			if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, path), []byte("first bytes\n"), 0644); err != nil {
				t.Fatal(err)
			}
			gitExec(t, dir, "add", "--", path)
			gitExec(t, dir, "commit", "-m", "first")
			first := testutil.RunGit(t, dir, "rev-parse", "HEAD")
			if err := os.WriteFile(filepath.Join(dir, path), []byte("clean bytes\n"), 0644); err != nil {
				t.Fatal(err)
			}
			gitExec(t, dir, "add", "--", path)
			gitExec(t, dir, "commit", "-m", "clean")
			gitExec(t, dir, "rm", "--", path)
			gitExec(t, dir, "commit", "-m", "remove")
			before := testutil.RunGit(t, dir, "status", "--porcelain=v1")
			blobs, err := UnpushedBlobs(dir, []string{path}, install.ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if len(blobs) != 2 {
				t.Fatalf("blobs=%+v", blobs)
			}
			found := false
			for _, blob := range blobs {
				if blob.Commit == first && blob.Path == path && string(blob.Data) == "first bytes\n" {
					found = true
				}
			}
			if !found {
				t.Fatal("initial bytes omitted")
			}
			if testutil.RunGit(t, dir, "status", "--porcelain=v1") != before {
				t.Fatal("history read changed index")
			}
		})
	}
}

func TestUnpushedBlobsUnbornAndPublished(t *testing.T) {
	dir := t.TempDir()
	gitExec(t, dir, "init")
	if blobs, err := UnpushedBlobs(dir, []string{"team.yaml"}, install.ParseOptions{}); err != nil || len(blobs) != 0 {
		t.Fatalf("unborn=%v %v", blobs, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "team.yaml"), []byte("published\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitExec(t, dir, "add", "team.yaml")
	gitExec(t, dir, "commit", "-m", "published")
	gitExec(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	if blobs, err := UnpushedBlobs(dir, []string{"team.yaml"}, install.ParseOptions{}); err != nil || len(blobs) != 0 {
		t.Fatalf("published=%v %v", blobs, err)
	}
}
