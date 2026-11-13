package config

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"maps"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSharedSaveProperty(t *testing.T) {
	for _, fold := range []bool{false, true} {
		for _, operation := range []string{"save", "prepare", "projects", "updates"} {
			t.Run(fmt.Sprint(fold, "/", operation), func(t *testing.T) {
				sb, _ := sharedFixture(t, fmt.Sprintf("preserve_tilde_on_save: %v\nextras: [{name: local, targets: []}]\n", fold), "version: 1\nextras: [{name: rules, source: extras/rules, targets: [{path: ~/rules}]}]\n")
				sharedPath := filepath.Join(filepath.Dir(sb.ConfigPath), "team.yaml")
				original := sha256.Sum256([]byte(sb.ReadFile(sharedPath)))
				random := rand.New(rand.NewSource(42))
				for i := 0; i < 12; i++ {
					cfg, err := LoadReadOnlyPath(sb.ConfigPath)
					if err != nil {
						t.Fatal(err)
					}
					cfg.PreserveTildeOnSave = fold
					enabled := random.Intn(2) == 0
					cfg.Updates.AutoCheck = &enabled
					if operation != "updates" {
						cfg.Ignore = []string{fmt.Sprintf("fixture-%d", random.Intn(10000))}
					}
					// Independently construct the base local-only payload plus the pointer.
					local := *cfg
					local.sharedLayer = nil
					local.Extras = append([]ExtraConfig(nil), cfg.Extras[1:]...)
					if operation == "projects" {
						local.Projects = maps.Clone(cfg.sharedProjectDeclarations())
						if local.Projects == nil {
							local.Projects = map[string]ManagedProject{}
						}
						local.Projects[filepath.Join(sb.Root, fmt.Sprintf("app-%d", i))] = ManagedProject{Targets: []string{"claude"}, Skills: &ResourceTargetConfig{}}
						local.projectOverlay = nil
					}
					payload := local.withoutProjectTargets()
					if fold {
						payload = payload.cloneForSave()
					}
					want, err := marshalConfigFile(sb.ConfigPath, payload, schemaComment)
					if err != nil {
						t.Fatal(err)
					}
					switch operation {
					case "save":
						err = cfg.Save()
					case "prepare":
						var c *SaveCandidate
						c, err = PrepareSave(sb.ConfigPath, cfg)
						if err == nil {
							err = c.Write()
						}
					case "projects":
						// A project mutation must pass through the same shared strip owner.
						var c *SaveCandidate
						c, err = PrepareProjectLayers(sb.ConfigPath, cfg, func(l *ProjectLayers) error {
							l.SetShared(filepath.Join(sb.Root, fmt.Sprintf("app-%d", i)), ManagedProject{Targets: []string{"claude"}, Skills: &ResourceTargetConfig{}})
							return nil
						})
						if err == nil {
							err = c.Write()
						}
					case "updates":
						err = SetAutoUpdateCheck(enabled)
					}
					if err != nil {
						t.Fatal(err)
					}
					got := []byte(sb.ReadFile(sb.ConfigPath))
					var stored struct {
						Extras []ExtraConfig `yaml:"extras"`
					}
					if err := yaml.Unmarshal(got, &stored); err != nil {
						t.Fatal(err)
					}
					if len(stored.Extras) != 1 || stored.Extras[0].Name != "local" {
						t.Fatal("shared declarations leaked")
					}
					if !bytes.Equal(got, want) {
						t.Fatalf("base bytes differ:\n%s\nwant:\n%s", got, want)
					}
					raw, err := os.ReadFile(sharedPath)
					if err != nil || sha256.Sum256(raw) != original {
						t.Fatal("shared bytes changed")
					}
				}
			})
		}
	}
}
