package git

import (
	"bytes"
	"fmt"
	"strings"

	"skillshare/internal/gitenv"
	"skillshare/internal/install"
)

type PathBlob struct {
	Commit, Path string
	Data         []byte
}

// UnpushedBlobs reads changed candidate blobs from every outgoing commit, not
// just HEAD. Combined merge diffs include new resolutions, not inherited data.
// NUL-delimited diff-tree names preserve spaces and Git's quoting characters.
func UnpushedBlobs(dir string, paths []string, opts install.ParseOptions) ([]PathBlob, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	revisions, stdin, rejected, err := unpushedRange(dir, opts)
	if err != nil {
		return nil, fmt.Errorf("check shared history: %w", err)
	}
	if rejected {
		return nil, nil
	}
	pathspec := make([]string, 0, len(paths))
	for _, path := range paths {
		pathspec = append(pathspec, ":(top,literal)"+path)
	}
	args := []string{"--no-replace-objects", "log", "--format=%x00%H", "--full-history", "--root", "-c", "--diff-filter=AMT", "--no-renames", "--no-show-signature", "--raw", "--no-abbrev"}
	args = append(args, revisions...)
	args = append(args, "--")
	args = append(args, pathspec...)
	cmd := gitenv.Command(args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("check shared history: %w", err)
	}
	var blobs []PathBlob
	for _, record := range strings.Split(string(raw), "\x00") {
		commit, diff, ok := strings.Cut(strings.TrimSpace(record), "\n")
		if !ok || !strings.HasPrefix(strings.TrimSpace(diff), ":") {
			continue
		}
		args := []string{"--no-replace-objects", "diff-tree", "--root", "-c", "--no-commit-id", "--name-only", "-z", "-r", "--diff-filter=AMT", "--no-renames", commit, "--"}
		args = append(args, pathspec...)
		names := gitenv.Command(args...)
		names.Dir = dir
		changed, err := names.Output()
		if err != nil {
			return nil, fmt.Errorf("check shared history paths: %w", err)
		}
		for _, path := range bytes.Split(changed, []byte{0}) {
			if len(path) == 0 {
				continue
			}
			blob := gitenv.Command("--no-replace-objects", "cat-file", "blob", commit+":"+string(path))
			blob.Dir = dir
			data, err := blob.Output()
			if err != nil {
				return nil, fmt.Errorf("read shared history blob: %w", err)
			}
			blobs = append(blobs, PathBlob{Commit: commit, Path: string(path), Data: data})
		}
	}
	return blobs, nil
}
