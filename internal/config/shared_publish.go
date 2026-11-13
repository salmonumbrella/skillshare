package config

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	gitops "skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/sharedfile"
)

func (c *Config) sharedStateForGit(gitDir string) (*SharedStateFile, error) {
	state, err := LoadSharedState(ConfigPath(), gitDir, filepath.Dir(c.SharedStatus().Path))
	if err != nil {
		return nil, &SharedError{Code: SharedPublication, Path: c.SharedStatus().Path, Cause: err}
	}
	return state, nil
}

func (c *Config) SharedGitPaths(gitDir string) ([]string, error) {
	if !c.SharedEnabled() {
		return nil, nil
	}
	state, err := c.sharedStateForGit(gitDir)
	if err != nil {
		return nil, err
	}
	paths := []string{sharedfile.DefaultName}
	seen := map[string]bool{sharedfile.DefaultName: true}
	candidates := append([]string{c.SharedStatus().Path}, state.State.Pointers...)
	for _, path := range candidates {
		rel, err := filepath.Rel(gitDir, path)
		if err != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		rel = filepath.ToSlash(rel)
		if !seen[rel] {
			seen[rel] = true
			paths = append(paths, rel)
		}
	}
	return paths, nil
}

func checkSharedDeclaration(doc *sharedfile.Document, unavailable *sharedfile.Unavailable) *sharedfile.Unavailable {
	if unavailable == nil {
		_, unavailable = decodeSharedExtras(doc)
	}
	return unavailable
}

func (c *Config) CheckSharedWorktree(gitDir string) error {
	if !c.SharedEnabled() {
		return nil
	}
	if err := c.CheckSharedApply(); err != nil {
		return err
	}
	paths, err := c.SharedGitPaths(gitDir)
	if err != nil {
		return err
	}
	// The active pointer is checked even when outside the publication root.
	absolute := []string{c.SharedStatus().Path}
	for _, path := range paths {
		absolute = append(absolute, filepath.Join(gitDir, filepath.FromSlash(path)))
	}
	seen := map[string]bool{}
	for _, path := range absolute {
		path = filepath.Clean(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		doc, unavailable := sharedfile.Open(path)
		if unavailable != nil && unavailable.Reason == sharedfile.Missing && path != filepath.Clean(c.SharedStatus().Path) {
			continue
		}
		if unavailable = checkSharedDeclaration(doc, unavailable); unavailable != nil {
			code := SharedPublication
			if path == filepath.Clean(c.SharedStatus().Path) {
				code = SharedUnavailable
			}
			return &SharedError{Code: code, Path: path, Cause: unavailable}
		}
	}
	return nil
}

func (c *Config) RecordSharedPointer(gitDir string, dryRun bool) error {
	if !c.SharedEnabled() {
		return nil
	}
	var state *SharedStateFile
	var err error
	if dryRun {
		state, err = c.sharedStateForGit(gitDir)
	} else {
		var release func()
		state, release, err = BeginSharedStateWrite(context.Background(), ConfigPath(), gitDir, filepath.Dir(c.SharedStatus().Path))
		if err == nil {
			defer release()
		}
	}
	if err != nil {
		return &SharedError{Code: SharedPublication, Path: c.SharedStatus().Path, Cause: err}
	}
	state.RecordPointer(c.SharedStatus().Path)
	if dryRun {
		return nil
	}
	if err := state.Save(); err != nil {
		return &SharedError{Code: SharedPublication, Path: c.SharedStatus().Path, Cause: err}
	}
	return nil
}

func (c *Config) CheckSharedOutgoing(gitDir string, opts install.ParseOptions) error {
	if !c.SharedEnabled() {
		return nil
	}
	paths, err := c.SharedGitPaths(gitDir)
	if err != nil {
		return err
	}
	blobs, err := gitops.UnpushedBlobs(gitDir, paths, opts)
	if err != nil {
		return &SharedError{Code: SharedPublication, Path: c.SharedStatus().Path, Cause: err}
	}
	for _, blob := range blobs {
		doc, unavailable := sharedfile.Check(blob.Path, blob.Data)
		if unavailable = checkSharedDeclaration(doc, unavailable); unavailable != nil {
			short := blob.Commit
			if len(short) > 7 {
				short = short[:7]
			}
			// The reason contains locations/rule names only, never Git's blob bytes.
			return &SharedError{Code: SharedPublication, Path: blob.Path, Cause: fmt.Errorf("commit %s: %w. Remove the unsafe shared file from every offending commit with git rebase -i (use --root for the initial commit), edit or remove the file and amend, then continue the rebase; a later clean commit does not remove earlier bytes; skillshare never rewrites history automatically", short, unavailable)}
		}
	}
	return nil
}
