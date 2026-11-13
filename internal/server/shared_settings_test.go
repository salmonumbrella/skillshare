package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/sharedfile"
	syncpkg "skillshare/internal/sync"
	"skillshare/internal/testutil"
)

func TestSharedUnavailableRelinkAPILeavesFilesAndConfigUnchanged(t *testing.T) {
	if !syncpkg.CanCreateFileLink() {
		t.Skip("requires file links")
	}
	for _, unavailable := range []string{"missing", "version"} {
		for _, action := range []string{"move", "as", "source"} {
			t.Run(unavailable+"/"+action, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
				source, target := filepath.Join(sb.Root, "source-extra"), filepath.Join(sb.Root, "target-extra")
				replacement := filepath.Join(sb.Root, "replacement")
				sb.WriteFile(filepath.Join(source, "fixture.md"), "old source\n")
				sb.WriteFile(filepath.Join(replacement, "fixture.md"), "new source\n")
				file := ""
				if action == "as" {
					file = "file: fixture.md,"
				}
				sb.WriteFile(shared, "version: 1\n")
				sb.WriteConfig(fmt.Sprintf("source: %q\nshared: {path: ./team.yaml}\ntargets: {}\nextras: [{name: rules, source: %q, %s targets: [{path: %q, mode: merge}]}]\n", filepath.ToSlash(sb.SourcePath), filepath.ToSlash(source), file, filepath.ToSlash(target)))
				cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				s := New(cfg, "127.0.0.1:0", "", "")
				if _, err := s.syncExtras("rules", false, false); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Readlink(filepath.Join(target, "fixture.md")); err != nil {
					t.Fatal("initial placement missing", err)
				}
				if unavailable == "missing" {
					if err := os.Remove(shared); err != nil {
						t.Fatal(err)
					}
				} else {
					sb.WriteFile(shared, "version: 2\n")
				}
				s.cfg, err = config.LoadReadOnlyPath(sb.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				// Walk the synthetic sandbox only, including state and backup folders.
				snapshot := func() map[string]string {
					files := map[string]string{}
					if err := filepath.WalkDir(sb.Root, func(path string, entry fs.DirEntry, err error) error {
						if err != nil {
							return err
						}
						info, err := entry.Info()
						if err != nil {
							return err
						}
						content := ""
						if entry.Type()&os.ModeSymlink != 0 {
							content, err = os.Readlink(path)
						} else if !entry.IsDir() {
							var raw []byte
							raw, err = os.ReadFile(path)
							content = string(raw)
						}
						files[path] = info.Mode().String() + ":" + content
						return err
					}); err != nil {
						t.Fatal(err)
					}
					return files
				}
				before := snapshot()
				configBefore, err := json.Marshal(s.cfg.Extras)
				if err != nil {
					t.Fatal(err)
				}
				body := fmt.Sprintf(`{"path":%q,"target":{"path":%q}}`, target, filepath.Join(sb.Root, "moved"))
				handler := (*Server).handleExtrasEditTarget
				if action == "as" {
					body = fmt.Sprintf(`{"path":%q,"target":{"path":%q,"as":"renamed.md"}}`, target, target)
				} else if action == "source" {
					body = fmt.Sprintf(`{"source":%q}`, replacement)
					handler = (*Server).handleExtrasEdit
				}
				r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
				r.SetPathValue("name", "rules")
				rr := httptest.NewRecorder()
				handler(s, rr, r)
				if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), config.SharedUnavailable) {
					t.Fatalf("response=%d %s", rr.Code, rr.Body.String())
				}
				if !reflect.DeepEqual(before, snapshot()) {
					t.Fatal("refused relink changed config, sources, targets, state or backups")
				}
				configAfter, err := json.Marshal(s.cfg.Extras)
				if err != nil || string(configBefore) != string(configAfter) {
					t.Fatal("refused relink mutated in-memory config")
				}
			})
		}
	}
}

func TestSharedUnavailableAllowsConfigOnlyExtraRepairs(t *testing.T) {
	for _, action := range []string{"target-settings", "rename", "same-source"} {
		t.Run(action, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source, target := filepath.Join(sb.Root, "source-extra"), filepath.Join(sb.Root, "target-extra")
			sb.WriteFile(filepath.Join(source, "fixture.md"), "source\n")
			sb.WriteFile(filepath.Join(target, "fixture.md"), "user file\n")
			sb.WriteConfig(fmt.Sprintf("source: %q\nshared: {path: ./missing.yaml}\ntargets: {}\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy}]}]\n", filepath.ToSlash(sb.SourcePath), filepath.ToSlash(source), filepath.ToSlash(target)))
			cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			s := New(cfg, "127.0.0.1:0", "", "")
			body := fmt.Sprintf(`{"path":%q,"target":{"path":%q,"mode":"copy","include":["fixture.md"],"flatten":true}}`, target, target)
			handler := (*Server).handleExtrasEditTarget
			if action == "rename" {
				body, handler = `{"name":"notes"}`, (*Server).handleExtrasEdit
			} else if action == "same-source" {
				body, handler = fmt.Sprintf(`{"source":%q}`, source), (*Server).handleExtrasEdit
			}
			r := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(body))
			r.SetPathValue("name", "rules")
			rr := httptest.NewRecorder()
			handler(s, rr, r)
			if rr.Code != http.StatusOK {
				t.Fatalf("repair=%d %s", rr.Code, rr.Body.String())
			}
			if sb.ReadFile(filepath.Join(source, "fixture.md")) != "source\n" || sb.ReadFile(filepath.Join(target, "fixture.md")) != "user file\n" {
				t.Fatal("config-only repair applied or pruned a target")
			}
			if action == "rename" && s.cfg.Extras[0].Name != "notes" || action == "target-settings" && !s.cfg.Extras[0].Targets[0].Flatten {
				t.Fatal("repair was not saved")
			}
			statePath, err := config.SharedStatePath(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(statePath); !os.IsNotExist(err) {
				t.Fatal("config-only repair accessed shared state")
			}
		})
	}
}

func TestSharedOwnedAPIRefusesBeforeSideEffects(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		handler    func(*Server, http.ResponseWriter, *http.Request)
	}{
		{"delete", `{}`, (*Server).handleExtrasDelete},
		{"create", `{"name":"policy","file":"AGENTS.md","targets":[{"path":"~/other","mode":"copy"}]}`, (*Server).handleExtrasCreate},
		{"edit", `{"name":"renamed"}`, (*Server).handleExtrasEdit},
		{"edit-target", `{"path":"~/placed","target":{"path":"~/other","mode":"copy"}}`, (*Server).handleExtrasEditTarget},
		{"mode", `{"target":"~/placed","mode":"copy"}`, (*Server).handleExtrasMode},
		{"add-target", `{"path":"~/other","mode":"copy"}`, (*Server).handleExtrasAddTarget},
		{"remove-target", `{"path":"~/placed"}`, (*Server).handleExtrasRemoveTarget},
		{"instruction-create", `{"name":"policy","content":"new content"}`, (*Server).handleCreateSharedInstructions},
		{"instruction-mode", `{"mode":"copy"}`, (*Server).handlePutSharedInstructionsMode},
		{"location-add", `{"path":"~/other/AGENTS.md","mode":"copy"}`, (*Server).handleAddSharedInstructionsLocation},
		{"location-delete", `{"path":"~/placed/AGENTS.md"}`, (*Server).handleDeleteSharedInstructionsLocation},
		{"resolve-collect", `{"target":"claude","action":"collect"}`, (*Server).handleResolveSharedInstructions},
		{"assign-removal", `{"targets":["claude"],"extras":[]}`, (*Server).handleAssignSharedInstructions},
		{"restore", `{"target":"claude"}`, (*Server).handleRestoreSharedInstructions},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
			source := filepath.Join(sb.Root, "policy")
			sb.WriteFile(filepath.Join(source, "AGENTS.md"), "source content\n")
			sb.WriteFile(filepath.Join(sb.Home, ".claude", "CLAUDE.md"), "own content\n")
			sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {claude: {path: ~/.claude/skills}}\n")
			sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: policy, source: %q, file: AGENTS.md, targets: [{path: ~/.claude, as: CLAUDE.md, mode: copy}]}]\n", filepath.ToSlash(source)))
			cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			s := New(cfg, "127.0.0.1:0", "", "")
			originalConfig, originalShared := sb.ReadFile(sb.ConfigPath), sb.ReadFile(shared)
			rr := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			r.SetPathValue("name", "policy")
			r.SetPathValue("target", "claude")
			tc.handler(s, rr, r)
			if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), `"error_code":"shared_owned"`) && !strings.Contains(rr.Body.String(), `"error_code": "shared_owned"`) {
				t.Fatalf("response=%d %s", rr.Code, rr.Body.String())
			}
			if sb.ReadFile(sb.ConfigPath) != originalConfig || sb.ReadFile(shared) != originalShared || sb.ReadFile(filepath.Join(source, "AGENTS.md")) != "source content\n" || sb.ReadFile(filepath.Join(sb.Home, ".claude", "CLAUDE.md")) != "own content\n" {
				t.Fatal("refused mutation changed a file")
			}
			if _, err := os.Stat(filepath.Join(sb.Home, "other")); !os.IsNotExist(err) {
				t.Fatal("refused mutation created target")
			}
		})
	}
}

func TestSharedUnavailableExtrasSyncAPI(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./missing.yaml}\ntargets: {}\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, "127.0.0.1:0", "", "")
	rr := httptest.NewRecorder()
	s.handleExtrasSync(rr, httptest.NewRequest(http.MethodPost, "/api/extras/sync", strings.NewReader(`{}`)))
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), config.SharedUnavailable) {
		t.Fatalf("response=%d %s", rr.Code, rr.Body.String())
	}
}

func TestSharedLocalOverrideRemainsWritable(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	sb.WriteFile(shared, "version: 1\nextras: [{name: rules, targets: [{path: ~/shared}]}]\n")
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\nextras: [{name: rules, targets: [{path: ~/local}]}]\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, "127.0.0.1:0", "", "")
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/extras/rules/targets", strings.NewReader(`{"path":"~/other","mode":"copy"}`))
	r.SetPathValue("name", "rules")
	s.handleExtrasAddTarget(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("override=%d %s", rr.Code, rr.Body.String())
	}
	if sb.ReadFile(shared) != "version: 1\nextras: [{name: rules, targets: [{path: ~/shared}]}]\n" {
		t.Fatal("override changed shared file")
	}
}

func TestSharedPublicationAPIRefusesBeforeStaging(t *testing.T) {
	for _, scope := range []string{"skills", "agents", "extras", "root"} {
		for _, push := range []bool{false, true} {
			t.Run(scope+fmt.Sprint(push), func(t *testing.T) {
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
				sb.WriteFile(filepath.Join(dir, "fixture.txt"), "fixture")
				testutil.RunGit(t, dir, "init")
				testutil.ConfigureGitUser(t, dir)
				testutil.RunGit(t, dir, "add", "-A")
				testutil.RunGit(t, dir, "commit", "-m", "initial fixture")
				testutil.RunGit(t, dir, "remote", "add", "origin", filepath.Join(sb.Root, "remote.git"))
				sb.WriteFile(shared, "version: 2\n")
				cfg, err = config.LoadReadOnlyPath(sb.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				s := New(cfg, "127.0.0.1:0", "", "")
				before := testutil.RunGit(t, dir, "diff", "--cached", "--raw")
				rr := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
				if push {
					s.handlePush(rr, r)
				} else {
					s.handleGitCommit(rr, r)
				}
				if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), config.SharedUnavailable) {
					t.Fatalf("response=%d %s", rr.Code, rr.Body.String())
				}
				if testutil.RunGit(t, dir, "diff", "--cached", "--raw") != before {
					t.Fatal("API staged before refusal")
				}
			})
		}
	}
}

func TestSharedExtrasAPILastRemoval(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	source := filepath.Join(sb.Root, "rules")
	target := filepath.Join(sb.Root, "placed")
	sb.WriteFile(filepath.Join(source, "fixture.md"), "managed\n")
	sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: rules,source: %q,targets: [{path: %q,mode: copy}]}]\n", filepath.ToSlash(source), filepath.ToSlash(target)))
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, "127.0.0.1:0", "", "")
	for _, remove := range []bool{false, true} {
		if remove {
			sb.WriteFile(shared, "version: 1\n")
		}
		rr := httptest.NewRecorder()
		s.handleExtrasSync(rr, httptest.NewRequest(http.MethodPost, "/api/extras/sync", strings.NewReader(`{}`)))
		if rr.Code != http.StatusOK {
			t.Fatalf("sync=%d %s", rr.Code, rr.Body.String())
		}
	}
	if _, err := os.Lstat(filepath.Join(target, "fixture.md")); !os.IsNotExist(err) {
		t.Fatal("API left last placement")
	}
}

func TestSharedPullAPIReloadsAndDetachesLastExtra(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\ngit_root: root\nshared: {path: ./team.yaml}\ntargets: {}\n")
	base := t.TempDir()
	remote := testutil.SetupBareRemoteRepo(t, base)
	testutil.SeedRemoteBranch(t, base, remote, "main", map[string]string{
		".gitignore":              "config.yaml\n",
		"team.yaml":               "version: 1\nextras: [{name: rules,source: ./extras/rules,targets: [{path: ~/placed,mode: copy}]}]\n",
		"extras/rules/fixture.md": "from remote\n",
	})
	root := filepath.Dir(sb.ConfigPath)
	testutil.RunGit(t, root, "init")
	testutil.ConfigureGitUser(t, root)
	testutil.RunGit(t, root, "remote", "add", "origin", remote)
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, "127.0.0.1:0", "", "")
	pull := func() {
		t.Helper()
		rr := httptest.NewRecorder()
		s.handlePull(rr, httptest.NewRequest(http.MethodPost, "/api/git/pull", strings.NewReader(`{}`)))
		if rr.Code != http.StatusOK {
			t.Fatalf("pull=%d %s", rr.Code, rr.Body.String())
		}
	}
	pull()
	if sb.ReadFile(filepath.Join(sb.Home, "placed", "fixture.md")) != "from remote\n" {
		t.Fatal("dashboard pull used stale declarations")
	}
	seed := filepath.Join(base, "seed-main")
	sb.WriteFile(filepath.Join(seed, "team.yaml"), "version: 1\n")
	testutil.RunGit(t, seed, "add", "team.yaml")
	testutil.RunGit(t, seed, "commit", "-m", "remove last declaration")
	testutil.RunGit(t, seed, "push")
	pull()
	if _, err := os.Lstat(filepath.Join(sb.Home, "placed", "fixture.md")); !os.IsNotExist(err) {
		t.Fatal("dashboard pull did not detach")
	}
}

func TestSharedErrorIncludesSafeTranslationParams(t *testing.T) {
	rr := httptest.NewRecorder()
	err := &config.SharedError{Code: config.SharedPublication, Path: "/team.yaml", Cause: &sharedfile.Unavailable{
		Path: "/team.yaml", Reason: sharedfile.Privacy, Findings: []sharedfile.Finding{{Line: 7, Entry: "rules", Rule: "secret"}},
	}}
	if !writeSharedError(rr, err) {
		t.Fatal("not handled")
	}
	var body struct {
		Params map[string]string `json:"error_params"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"path": "/team.yaml", "reason": "privacy", "line": "7", "entry": "rules", "rule": "secret"} {
		if body.Params[key] != want {
			t.Errorf("%s=%q want %q", key, body.Params[key], want)
		}
	}
}

func TestNamedSharedExtrasSyncProtectsEditedReplacement(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, target := filepath.Join(sb.Root, "rules"), filepath.Join(sb.Root, "placed")
	shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
	sb.WriteFile(filepath.Join(source, "fixture.md"), "managed\n")
	sb.WriteFile(shared, fmt.Sprintf("version: 1\nextras: [{name: rules, source: %q, targets: [{path: %q, mode: copy}]}]\n", filepath.ToSlash(source), filepath.ToSlash(target)))
	sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\n")
	cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, "127.0.0.1:0", "", "")
	if _, err := s.syncExtras("rules", false, false); err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(target, "fixture.md"), "edited\n")
	replacement := filepath.Join(sb.Root, "replacement")
	sb.WriteFile(filepath.Join(replacement, "fixture.md"), "replacement\n")
	sb.WriteFile(shared, strings.ReplaceAll(sb.ReadFile(shared), filepath.ToSlash(source), filepath.ToSlash(replacement)))
	results, err := s.syncExtras("rules", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if sb.ReadFile(filepath.Join(target, "fixture.md")) != "edited\n" {
		t.Fatal("named apply overwrote protected output")
	}
	if len(results) != 1 || len(results[0].Targets) != 2 || len(results[0].Targets[0].Warnings) == 0 || results[0].Targets[1].Skipped != 1 {
		t.Fatalf("results=%+v", results)
	}
}
