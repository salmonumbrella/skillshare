package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

func TestHandlePutConfigSharedPointerHistory(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		t.Run(map[bool]string{false: "history", true: "unsafe state"}[unsafe], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			old := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
			next := filepath.Join(sb.Root, "new", "team.yaml")
			sb.WriteFile(old, "version: 1\n")
			sb.WriteFile(next, "version: 1\n")
			sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\n")
			cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			s := New(cfg, "127.0.0.1:0", "", "")
			if unsafe {
				t.Setenv("XDG_STATE_HOME", sb.SourcePath)
			}
			original := sb.ReadFile(sb.ConfigPath)
			raw := "# exact raw editor bytes\nsource: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: '" + filepath.ToSlash(next) + "'}\ntargets: {}\n"
			body, err := json.Marshal(map[string]string{"raw": raw})
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			s.handlePutConfig(rr, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(string(body))))
			if unsafe {
				if rr.Code != http.StatusInternalServerError || sb.ReadFile(sb.ConfigPath) != original {
					t.Fatalf("unsafe PUT wrote config: %d %s", rr.Code, rr.Body.String())
				}
				return
			}
			if rr.Code != http.StatusOK || sb.ReadFile(sb.ConfigPath) != raw {
				t.Fatalf("PUT=%d %s", rr.Code, rr.Body.String())
			}
			state, err := config.LoadSharedState(sb.ConfigPath, sb.SourcePath, filepath.Dir(next))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(state.State.Pointers, old) || !slices.Contains(state.State.Pointers, next) {
				t.Fatal(state.State.Pointers)
			}
		})
	}
}

func TestHandlePutConfigRepairsMalformedNoPointerVariants(t *testing.T) {
	for _, old := range []string{
		"# shared: {path: ignored.yaml}\ntargets: [invalid YAML\n",
		"note: \"shared: {path: ignored.yaml}\"\ntargets: [invalid YAML\n",
		"note: {shared: {path: ignored.yaml}}\ntargets: [invalid YAML\n",
	} {
		t.Run(old, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\ntargets: {}\n")
			cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			s := New(cfg, "127.0.0.1:0", "", "")
			sb.WriteConfig(old)
			path, _ := config.SharedStatePath(sb.ConfigPath)
			sb.WriteFile(path, "must not be read")
			raw := "# exact repair bytes\nsource: " + filepath.ToSlash(sb.SourcePath) + "\ntargets: {}\n"
			body, err := json.Marshal(map[string]string{"raw": raw})
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			s.handlePutConfig(rr, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(string(body))))
			if rr.Code != http.StatusOK || sb.ReadFile(sb.ConfigPath) != raw || sb.ReadFile(path) != "must not be read" {
				t.Fatalf("repair=%d %s", rr.Code, rr.Body.String())
			}
		})
	}
}
