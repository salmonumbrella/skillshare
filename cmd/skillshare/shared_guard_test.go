package main

import (
	"errors"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

func TestSharedPublicationCLIRefusesBeforeStaging(t *testing.T) {
	for _, scope := range []string{"skills", "agents", "extras", "root"} {
		for _, command := range []string{"commit", "push"} {
			for _, dryRun := range []bool{false, true} {
				t.Run(scope+"/"+command+map[bool]string{false: "", true: "/dry-run"}[dryRun], func(t *testing.T) {
					sb := testutil.NewSandbox(t)
					defer sb.Cleanup()
					shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
					sb.WriteFile(shared, "version: 1\n")
					sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ngit_root: " + scope + "\ntargets: {}\n")
					cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
					if err != nil {
						t.Fatal(err)
					}
					dir := cfg.EffectiveGitRoot()
					sb.WriteFile(filepath.Join(dir, "fixture.txt"), "fixture\n")
					testutil.RunGit(t, dir, "init")
					testutil.ConfigureGitUser(t, dir)
					testutil.RunGit(t, dir, "add", "-A")
					testutil.RunGit(t, dir, "commit", "-m", "initial fixture")
					testutil.RunGit(t, dir, "remote", "add", "origin", filepath.Join(sb.Root, "remote.git"))
					// Already staged user changes and tracked config must survive the refusal.
					sb.WriteFile(filepath.Join(dir, "fixture.txt"), "user staged\n")
					testutil.RunGit(t, dir, "add", "fixture.txt")
					sb.WriteFile(shared, "version: 2\n")
					before := testutil.RunGit(t, dir, "diff", "--cached", "--raw")
					head := testutil.RunGit(t, dir, "rev-parse", "HEAD")
					args := []string{}
					if dryRun {
						args = append(args, "--dry-run")
					}
					if command == "push" {
						err = cmdPush(args)
					} else {
						err = cmdCommit(args)
					}
					var refusal *config.SharedError
					if !errors.As(err, &refusal) || refusal.Code != config.SharedUnavailable {
						t.Fatalf("refusal=%v", err)
					}
					if testutil.RunGit(t, dir, "diff", "--cached", "--raw") != before || testutil.RunGit(t, dir, "rev-parse", "HEAD") != head {
						t.Fatal("publication refusal staged or committed")
					}
				})
			}
		}
	}
}
