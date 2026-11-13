package install

import "testing"

func TestTargetOverrides_EmptyListSurvivesSave(t *testing.T) {
	dir := t.TempDir()
	store := NewMetadataStore()
	store.SetTargetOverride("_repo/skill", nil)
	if err := store.Save(dir); err != nil {
		t.Fatal(err)
	}

	targets, ok := LoadTargetOverrides(dir)["_repo/skill"]
	if !ok || len(targets) != 0 {
		t.Errorf("want explicit empty override, got %v (present=%v)", targets, ok)
	}
}

func TestRemoveByNames_RemovesRepoTargetOverrides(t *testing.T) {
	store := NewMetadataStore()
	store.SetTargetOverride("_repo/skills/a", []string{"claude"})
	store.SetTargetOverride("_repo-other/b", []string{"cursor"})

	store.RemoveByNames(map[string]bool{"_repo": true})

	if _, ok := store.TargetOverrides["_repo/skills/a"]; ok {
		t.Error("override of the uninstalled repo must be removed")
	}
	if _, ok := store.TargetOverrides["_repo-other/b"]; !ok {
		t.Error("override of a sibling repo with a shared prefix must survive")
	}
}
