package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

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
