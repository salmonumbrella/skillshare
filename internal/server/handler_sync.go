package server

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/skillignore"
	ssync "skillshare/internal/sync"
)

// ignorePayload builds the common ignored-skills fields for JSON responses.
func ignorePayload(stats *skillignore.IgnoreStats) map[string]any {
	skills := []string{}
	rootFile := ""
	repoFiles := []string{}
	if stats != nil {
		if len(stats.IgnoredSkills) > 0 {
			skills = stats.IgnoredSkills
		}
		rootFile = stats.RootFile
		if stats.RepoFiles != nil {
			repoFiles = stats.RepoFiles
		}
	}
	return map[string]any{
		"ignored_count":  len(skills),
		"ignored_skills": skills,
		"ignore_root":    rootFile,
		"ignore_repos":   repoFiles,
	}
}

type syncTargetResult struct {
	Resource   string   `json:"resource"`
	Target     string   `json:"target"`
	Linked     []string `json:"linked"`
	Updated    []string `json:"updated"`
	Skipped    []string `json:"skipped"`
	Pruned     []string `json:"pruned"`
	DirCreated string   `json:"dir_created,omitempty"`
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		DryRun    bool     `json:"dryRun"`
		Force     bool     `json:"force"`
		Resources []string `json:"resources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if err != io.EOF {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	resources, err := parseServerSyncResources(body.Resources)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	globalMode := s.cfg.Mode
	if globalMode == "" {
		globalMode = "merge"
	}

	// Pre-check warnings via shared config validation
	warnings, validErr := config.ValidateConfig(s.cfg)
	if validErr != nil {
		writeError(w, http.StatusBadRequest, validErr.Error())
		return
	}

	// Discover skills once for all targets
	allSkills, ignoreStats, err := ssync.DiscoverSourceSkillsWithStats(s.cfg.Source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	if len(allSkills) == 0 {
		warnings = append(warnings, "source directory is empty (0 skills)")
	}

	// Registry entries are managed by install/uninstall, not sync.
	// Sync only manages symlinks — it must not prune registry entries
	// for installed skills whose files may be missing from disk.

	results := make([]syncTargetResult, 0)

	for name, target := range s.cfg.Targets {
		syncErrArgs := map[string]any{
			"targets_total":  len(s.cfg.Targets),
			"targets_failed": 1,
			"target":         name,
			"dry_run":        body.DryRun,
			"force":          body.Force,
			"scope":          "ui",
		}

		if resources.skills {
			sc := target.SkillsConfig()
			mode := sc.Mode
			if mode == "" {
				mode = globalMode
			}

			res := newSyncTargetResult(name, "skills")
			switch mode {
			case "merge":
				mergeResult, err := ssync.SyncTargetMergeWithSkills(name, target, allSkills, s.cfg.Source, body.DryRun, body.Force, s.projectRoot)
				if err != nil {
					s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
					writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
					return
				}
				res.Linked = mergeResult.Linked
				res.Updated = mergeResult.Updated
				res.Skipped = mergeResult.Skipped
				res.DirCreated = mergeResult.DirCreated

				pruneResult, err := ssync.PruneOrphanLinksWithSkills(ssync.PruneOptions{
					TargetPath: sc.Path, SourcePath: s.cfg.Source, Skills: allSkills,
					Include: sc.Include, Exclude: sc.Exclude, TargetNaming: sc.TargetNaming, TargetName: name,
					DryRun: body.DryRun, Force: body.Force,
				})
				if err == nil {
					res.Pruned = pruneResult.Removed
				}

			case "copy":
				copyResult, err := ssync.SyncTargetCopyWithSkills(name, target, allSkills, s.cfg.Source, body.DryRun, body.Force, nil)
				if err != nil {
					s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
					writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
					return
				}
				res.Linked = copyResult.Copied
				res.Updated = copyResult.Updated
				res.Skipped = copyResult.Skipped
				res.DirCreated = copyResult.DirCreated

				pruneResult, err := ssync.PruneOrphanCopiesWithSkills(sc.Path, allSkills, sc.Include, sc.Exclude, name, sc.TargetNaming, body.DryRun)
				if err == nil {
					res.Pruned = pruneResult.Removed
				}

			default:
				err := ssync.SyncTarget(name, target, s.cfg.Source, body.DryRun, s.projectRoot)
				if err != nil {
					s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
					writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
					return
				}
				res.Linked = []string{"(symlink mode)"}
			}
			results = append(results, res)
		}

		if resources.rules {
			res, err := s.syncManagedRulesForTarget(name, target, body.DryRun)
			if err != nil {
				s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
				writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
				return
			}
			results = append(results, res)
		}

		if resources.hooks {
			res, err := s.syncManagedHooksForTarget(name, target, body.DryRun)
			if err != nil {
				s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
				writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
				return
			}
			results = append(results, res)
		}
	}

	// Log the sync operation
	s.writeOpsLog("sync", "ok", start, map[string]any{
		"targets_total":  len(s.cfg.Targets),
		"targets_failed": 0,
		"dry_run":        body.DryRun,
		"force":          body.Force,
		"scope":          "ui",
	}, "")

	resp := map[string]any{
		"results":  results,
		"warnings": warnings,
	}
	maps.Copy(resp, ignorePayload(ignoreStats))
	writeJSON(w, resp)
}

func newSyncTargetResult(target, resource string) syncTargetResult {
	return syncTargetResult{
		Resource: resource,
		Target:   target,
		Linked:   make([]string, 0),
		Updated:  make([]string, 0),
		Skipped:  make([]string, 0),
		Pruned:   make([]string, 0),
	}
}

type diffItem struct {
	Skill  string `json:"skill"`
	Action string `json:"action"` // "link", "update", "skip", "prune", "local"
	Reason string `json:"reason"` // human-readable description
}

type diffTarget struct {
	Target         string     `json:"target"`
	Items          []diffItem `json:"items"`
	SkippedCount   int        `json:"skippedCount,omitempty"`
	CollisionCount int        `json:"collisionCount,omitempty"`
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before slow I/O.
	s.mu.RLock()
	source := s.cfg.Source
	globalMode := s.cfg.Mode
	targets := s.cloneTargets()
	s.mu.RUnlock()

	if globalMode == "" {
		globalMode = "merge"
	}

	filterTarget := r.URL.Query().Get("target")

	discovered, ignoreStats, err := ssync.DiscoverSourceSkillsWithStats(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	diffs := make([]diffTarget, 0)
	for name, target := range targets {
		if filterTarget != "" && filterTarget != name {
			continue
		}
		diffs = append(diffs, s.computeTargetDiff(name, target, discovered, globalMode, source))
	}

	resp := map[string]any{"diffs": diffs}
	maps.Copy(resp, ignorePayload(ignoreStats))
	writeJSON(w, resp)
}
