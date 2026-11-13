package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/config"
	syncpkg "skillshare/internal/sync"
	"skillshare/internal/testutil"
)

// Exercise the supported route after placement, then a local override without
// another sync: its previous shared receipt still owns the native restore proof.
func TestSharedReceiptRelinkAPI(t *testing.T) {
	for _, mode := range []string{"copy", "merge", "import", "prepend", "append"} {
		for _, action := range []string{"move", "as", "source", "rename-source", "proof-refusal"} {
			for _, edited := range []bool{false, true} {
				if action == "proof-refusal" && (mode != "copy" || edited) {
					continue
				}
				t.Run(fmt.Sprintf("%s/%s/edited=%v", mode, action, edited), func(t *testing.T) {
					if mode == "merge" && !syncpkg.CanCreateFileLink() {
						t.Skip("requires file links")
					}
					if edited && mode == "import" {
						t.Skip("import removes only its managed line; no content body to edit")
					}
					sb := testutil.NewSandbox(t)
					defer sb.Cleanup()
					shared := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
					source, replacement := filepath.Join(sb.Root, "rules"), filepath.Join(sb.Root, "replacement")
					target, sibling := filepath.Join(sb.Root, "placed"), filepath.Join(sb.Root, "sibling")
					native := filepath.Join(sb.Root, "native-only")
					file := "AGENTS.md"
					sb.WriteFile(filepath.Join(source, file), "managed\n")
					sb.WriteFile(filepath.Join(replacement, file), "replacement\n")
					for _, dir := range []string{target, sibling, native} {
						sb.WriteFile(filepath.Join(dir, file), "original\n")
					}
					declaration := fmt.Sprintf("{name: rules, source: %q, file: AGENTS.md, targets: [{path: %q, mode: %s}, {path: %q, mode: %s}]}", filepath.ToSlash(source), filepath.ToSlash(target), mode, filepath.ToSlash(sibling), mode)
					unrelated := fmt.Sprintf("{name: other, source: %q, file: AGENTS.md, targets: [{path: %q, mode: copy}]}", filepath.ToSlash(source), filepath.ToSlash(filepath.Join(sb.Root, "other")))
					sb.WriteFile(shared, "version: 1\nextras: ["+declaration+", "+unrelated+"]\n")
					base := "source: " + filepath.ToSlash(sb.SourcePath) + "\nshared: {path: ./team.yaml}\ntargets: {}\n"
					sb.WriteConfig(base)
					cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
					if err != nil {
						t.Fatal(err)
					}
					s := New(cfg, "127.0.0.1:0", "", "")
					if rr := serveExtras(s, http.MethodPost, "/api/extras/sync", `{}`); rr.Code != http.StatusOK {
						t.Fatal(rr.Body.String())
					}
					if action == "proof-refusal" {
						// A supported native restore clears its markers. A still-held
						// shared receipt must then refuse rather than guess new proof.
						if _, err := syncpkg.RestoreExtraTarget(syncpkg.NewExtraFile(source, file, target, "", mode)); err != nil {
							t.Fatal(err)
						}
						sb.WriteConfig(base + "extras: [" + declaration + "]\n")
						rr := serveExtras(s, http.MethodPatch, "/api/extras/rules", fmt.Sprintf(`{"source":%q}`, replacement))
						if rr.Code != http.StatusInternalServerError || sb.ReadFile(filepath.Join(target, file)) != "original\n" || sb.ReadFile(filepath.Join(sibling, file)) == "replacement\n" {
							t.Fatalf("replacement followed proof refusal: %d %s", rr.Code, rr.Body.String())
						}
						return
					}
					loadState := func() *config.SharedStateFile {
						t.Helper()
						state, err := config.LoadSharedState(sb.ConfigPath, s.cfg.EffectiveGitRoot(), filepath.Dir(shared))
						if err != nil {
							t.Fatal(err)
						}
						return state
					}
					before := loadState().State.Extras
					if len(before) != 2 || len(before[0].Receipt.Targets) != 2 || len(before[0].Receipt.Targets[0].Native) == 0 {
						t.Fatalf("placement did not record native proof: %+v", before)
					}
					// Native-only output joins the override, without acquiring a receipt.
					if result, err := syncpkg.SyncExtraFile(syncpkg.NewExtraFile(source, file, native, "", mode), false, ""); err != nil || len(result.Errors) > 0 {
						t.Fatalf("native placement: %v %+v", err, result)
					}
					local := strings.TrimSuffix(declaration, "]}") + fmt.Sprintf(", {path: %q, mode: %s}]}", filepath.ToSlash(native), mode)
					// A move selects only its old target. Source relink selects all three.
					if action == "rename-source" {
						base = strings.Replace(base, "path: ./team.yaml", "path: ./team.yaml, skip: {extras: [rules]}", 1)
					}
					sb.WriteConfig(base + "extras: [" + local + "]\n")
					oldOutput := filepath.Join(target, file)
					kept := "edited by user\n"
					if edited {
						if mode == "prepend" || mode == "append" {
							kept = strings.Replace(sb.ReadFile(oldOutput), "managed\n", kept, 1)
						}
						// Replacing a link with a regular file is a supported user edit.
						if err := os.Remove(oldOutput); err != nil {
							t.Fatal(err)
						}
						sb.WriteFile(oldOutput, kept)
					}
					method, route, nextName := http.MethodPut, "/api/extras/rules/targets", "rules"
					newOutput := filepath.Join(sb.Root, "moved", file)
					body := fmt.Sprintf(`{"path":%q,"target":{"path":%q,"mode":%q}}`, target, filepath.Dir(newOutput), mode)
					if action == "as" {
						newOutput = filepath.Join(target, "RENAMED.md")
						body = fmt.Sprintf(`{"path":%q,"target":{"path":%q,"as":"RENAMED.md","mode":%q}}`, target, target, mode)
					} else if strings.Contains(action, "source") {
						method, route, newOutput = http.MethodPatch, "/api/extras/rules", oldOutput
						rename := ""
						if action == "rename-source" {
							rename, nextName = `"name":"renamed",`, "renamed"
						}
						body = fmt.Sprintf(`{%s"source":%q}`, rename, replacement)
					}
					rr := serveExtras(s, method, route, body)
					if rr.Code != http.StatusOK {
						t.Fatalf("relink=%d %s", rr.Code, rr.Body.String())
					}
					state := loadState().State
					if state.Pending != nil || len(state.Extras) == 0 || !reflect.DeepEqual(state.Extras[len(state.Extras)-1], before[1]) {
						t.Fatalf("unrelated receipt changed: %+v", state)
					}
					if action == "move" || action == "as" {
						if len(state.Extras) != 2 || len(state.Extras[0].Receipt.Targets) != 1 || !reflect.DeepEqual(state.Extras[0].Receipt.Targets[0], before[0].Receipt.Targets[1]) {
							t.Fatalf("unselected receipt changed: %+v", state.Extras)
						}
						want := "original\n"
						if edited {
							want = kept
						}
						if got := sb.ReadFile(oldOutput); got != want {
							t.Fatalf("old output=%q want %q", got, want)
						}
					} else {
						if len(state.Extras) != 1 {
							t.Fatalf("selected receipts stranded: %+v", state.Extras)
						}
						if got := sb.ReadFile(filepath.Join(native, file)); strings.Contains(got, "managed\n") || !strings.Contains(got, "replacement") && mode != "import" {
							t.Fatalf("native-only target not relinked: %q", got)
						}
					}
					if edited && newOutput == oldOutput {
						if sb.ReadFile(oldOutput) != kept || !strings.Contains(rr.Body.String(), "kept:") {
							t.Fatal("edited output not protected/reported")
						}
					} else if got := sb.ReadFile(newOutput); mode != "import" && !strings.Contains(got, map[bool]string{true: "replacement", false: "managed"}[strings.Contains(action, "source")]) {
						t.Fatalf("replacement missing: %q", got)
					}
					// A fresh routed retry verifies release and retained proof. Edited
					// occupants are checked in this run; their receipt is now relinquished.
					if !edited {
						rr = serveExtras(s, http.MethodPost, "/api/extras/sync", fmt.Sprintf(`{"name":%q}`, nextName))
						if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), "native restore metadata changed") {
							t.Fatalf("fresh retry=%d %s", rr.Code, rr.Body.String())
						}
						if strings.Contains(action, "source") {
							if _, err := syncpkg.RestoreExtraTarget(syncpkg.NewExtraFile(replacement, file, target, "", mode)); err != nil || sb.ReadFile(oldOutput) != "original\n" {
								t.Fatalf("replacement lost original restore point: %v", err)
							}
						}
					}
				})
			}
		}
	}
}

func TestRelinkNativeCleanupWithoutSharedState(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source, target := filepath.Join(sb.Root, "rules"), filepath.Join(sb.Root, "placed")
			sb.WriteFile(filepath.Join(source, "AGENTS.md"), "managed\n")
			sb.WriteFile(filepath.Join(target, "AGENTS.md"), "original\n")
			extra := config.ExtraConfig{Name: "rules", Source: source, File: "AGENTS.md", Targets: []config.ExtraTargetConfig{{Path: target, Mode: "copy"}}}
			sb.WriteConfig("source: " + filepath.ToSlash(sb.SourcePath) + "\ntargets: {}\n")
			cfg, err := config.LoadReadOnlyPath(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			cfg.Extras = []config.ExtraConfig{extra}
			s := New(cfg, "", "", "")
			if project {
				pcfg := &config.ProjectConfig{Extras: cfg.Extras}
				pcfg.Extras[0].Source = "rules"
				if err := pcfg.Save(sb.Root); err != nil {
					t.Fatal(err)
				}
				s = NewProject(cfg, pcfg, sb.Root, "", "", "")
			} else if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			if _, err := syncpkg.SyncExtraFile(syncpkg.NewExtraFile(source, extra.File, target, "", "copy"), false, ""); err != nil {
				t.Fatal(err)
			}
			body, err := json.Marshal(map[string]any{"path": target, "target": map[string]string{"path": filepath.Join(sb.Root, "moved"), "mode": "copy"}})
			if err != nil {
				t.Fatal(err)
			}
			rr := serveExtras(s, http.MethodPut, "/api/extras/rules/targets", string(body))
			if rr.Code != http.StatusOK || sb.ReadFile(filepath.Join(target, "AGENTS.md")) != "original\n" || sb.ReadFile(filepath.Join(sb.Root, "moved", "AGENTS.md")) != "managed\n" {
				t.Fatalf("baseline relink=%d %s", rr.Code, rr.Body.String())
			}
			statePath, err := config.SharedStatePath(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(statePath); !os.IsNotExist(err) {
				t.Fatal("no-pointer/project relink accessed shared state")
			}
		})
	}
}
