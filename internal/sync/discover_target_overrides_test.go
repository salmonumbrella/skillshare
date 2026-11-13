package sync

import (
	"path/filepath"
	"reflect"
	"testing"

	"skillshare/internal/install"
)

// setupTrackedSkills creates a tracked-repo skill and a regular skill, both
// targeting cursor in frontmatter, plus the given overrides in .metadata.json.
func setupTrackedSkills(t *testing.T, overrides map[string][]string) string {
	t.Helper()
	src := t.TempDir()
	fm := "---\nname: s\nmetadata:\n  targets:\n    - cursor\n---\n# s"
	writeSkillMD(t, filepath.Join(src, "_repo", "skills", "tracked"), fm)
	writeSkillMD(t, filepath.Join(src, "local"), fm)
	if overrides != nil {
		store := install.NewMetadataStore()
		for rel, targets := range overrides {
			store.SetTargetOverride(rel, targets)
		}
		if err := store.Save(src); err != nil {
			t.Fatal(err)
		}
	}
	return src
}

func discoveredTargets(t *testing.T, src string) map[string][]string {
	t.Helper()
	skills, err := DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, s := range skills {
		got[s.RelPath] = s.Targets
	}
	return got
}

func TestDiscover_TargetOverrideReplacesTrackedFrontmatter(t *testing.T) {
	src := setupTrackedSkills(t, map[string][]string{"_repo/skills/tracked": {"claude"}})

	got := discoveredTargets(t, src)
	if !reflect.DeepEqual(got["_repo/skills/tracked"], []string{"claude"}) {
		t.Errorf("tracked skill: want [claude], got %v", got["_repo/skills/tracked"])
	}
}

func TestDiscover_EmptyTargetOverrideMeansAllTargets(t *testing.T) {
	src := setupTrackedSkills(t, map[string][]string{"_repo/skills/tracked": nil})

	if got := discoveredTargets(t, src)["_repo/skills/tracked"]; got != nil {
		t.Errorf("tracked skill: want nil (all targets), got %v", got)
	}
}

func TestDiscover_NoTargetOverrideUsesFrontmatter(t *testing.T) {
	src := setupTrackedSkills(t, nil)

	if got := discoveredTargets(t, src)["_repo/skills/tracked"]; !reflect.DeepEqual(got, []string{"cursor"}) {
		t.Errorf("tracked skill: want [cursor], got %v", got)
	}
}

func TestDiscover_TargetOverrideIgnoredForRegularSkill(t *testing.T) {
	src := setupTrackedSkills(t, map[string][]string{"local": {"claude"}})

	if got := discoveredTargets(t, src)["local"]; !reflect.DeepEqual(got, []string{"cursor"}) {
		t.Errorf("regular skill: want frontmatter [cursor], got %v", got)
	}
}
