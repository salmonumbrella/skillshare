package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

// writeSizedSkill creates a skill whose description and body each contribute
// roughly `tokens` estimated tokens (EstimateTokens is ascii chars / 4).
func writeSizedSkill(t *testing.T, sourceDir, name string, tokens int) {
	t.Helper()
	skillDir := filepath.Join(sourceDir, name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill %s: %v", name, err)
	}
	content := "---\nname: " + name + "\ndescription: " + strings.Repeat("d ", tokens*2) + "\n---\n" + strings.Repeat("x ", tokens*2)
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write skill %s: %v", name, err)
	}
}

func doSync(t *testing.T, s *Server) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"dryRun":false}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/sync: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	return rr.Body.Bytes()
}

type contextCostWarning struct {
	Target       string
	TopOffenders []struct {
		Name   string `json:"name"`
		Tokens int    `json:"tokens"`
	} `json:"top_offenders"`
}

func contextCostWarnings(t *testing.T, body []byte) map[string]contextCostWarning {
	t.Helper()
	var resp struct {
		ContextCost *struct {
			Warnings []struct {
				Type         string `json:"type"`
				Target       string `json:"target"`
				TopOffenders []struct {
					Name   string `json:"name"`
					Tokens int    `json:"tokens"`
				} `json:"top_offenders"`
			} `json:"warnings"`
		} `json:"context_cost"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal sync response: %v", err)
	}
	if resp.ContextCost == nil {
		t.Fatalf("expected context_cost in response, got: %s", body)
	}
	warns := make(map[string]contextCostWarning, len(resp.ContextCost.Warnings))
	for _, w := range resp.ContextCost.Warnings {
		warns[w.Type] = contextCostWarning{Target: w.Target, TopOffenders: w.TopOffenders}
	}
	return warns
}

// Both targets tie, but the warning must choose one deterministic target and
// report offenders belonging to that target.
func TestHandleSync_ContextCostWarningNamesTiedTargets(t *testing.T) {
	s, src := newTestServerWithTargets(t, map[string]string{
		"alpha": filepath.Join(t.TempDir(), "alpha-skills"),
		"beta":  filepath.Join(t.TempDir(), "beta-skills"),
	})
	writeSizedSkill(t, src, "big-a", 1000)
	writeSizedSkill(t, src, "big-b", 1000)
	alpha := s.cfg.Targets["alpha"]
	alpha.EnsureSkills().Exclude = []string{"big-b"}
	s.cfg.Targets["alpha"] = alpha
	beta := s.cfg.Targets["beta"]
	beta.EnsureSkills().Exclude = []string{"big-a"}
	s.cfg.Targets["beta"] = beta

	s.cfg.ContextBudget = config.ContextBudgetConfig{
		WarnAlwaysLoadedTokens: intPtr(10),
		WarnOnDemandTokens:     intPtr(10),
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	warns := contextCostWarnings(t, doSync(t, s))
	if len(warns) != 2 {
		t.Fatalf("expected 2 budget warnings (always_loaded + on_demand), got %d: %v", len(warns), warns)
	}
	for _, typ := range []string{"always_loaded", "on_demand"} {
		warning, ok := warns[typ]
		if !ok {
			t.Errorf("missing %s warning", typ)
			continue
		}
		if warning.Target != "alpha" {
			t.Errorf("%s warning: expected target %q, got %q", typ, "alpha", warning.Target)
		}
		if len(warning.TopOffenders) == 0 || warning.TopOffenders[0].Name != "big-a" {
			t.Errorf("%s warning: expected top offender %q from alpha, got %+v", typ, "big-a", warning.TopOffenders)
		}
	}
}

// beta excludes the big skill, so alpha alone is the worst offender and the
// warning must name only it.
func TestHandleSync_ContextCostWarningNamesWorstTarget(t *testing.T) {
	s, src := newTestServerWithTargets(t, map[string]string{
		"alpha": filepath.Join(t.TempDir(), "alpha-skills"),
		"beta":  filepath.Join(t.TempDir(), "beta-skills"),
	})
	writeSizedSkill(t, src, "big", 1000)
	writeSizedSkill(t, src, "small", 5)
	beta := s.cfg.Targets["beta"]
	beta.EnsureSkills().Exclude = []string{"big"}
	s.cfg.Targets["beta"] = beta

	s.cfg.ContextBudget = config.ContextBudgetConfig{
		WarnAlwaysLoadedTokens: intPtr(10),
		WarnOnDemandTokens:     intPtr(10),
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	warns := contextCostWarnings(t, doSync(t, s))
	if len(warns) != 2 {
		t.Fatalf("expected 2 budget warnings (always_loaded + on_demand), got %d: %v", len(warns), warns)
	}
	for _, typ := range []string{"always_loaded", "on_demand"} {
		warning, ok := warns[typ]
		if !ok {
			t.Errorf("missing %s warning", typ)
			continue
		}
		if warning.Target != "alpha" {
			t.Errorf("%s warning: expected target %q, got %q", typ, "alpha", warning.Target)
		}
	}
}
