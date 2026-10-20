package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/projects"
	"skillshare/internal/ui"
)

func runProject(o projectOptions) error {
	start := time.Now()
	path := config.ReadOnlyConfigPath()
	cfg, err := config.LoadReadOnly()
	if err != nil {
		return err
	}
	service := projects.NewService(path, func() (*config.Config, error) { return cfg, nil })
	if o.action == "list" {
		items, err := service.List()
		if err != nil {
			return err
		}
		if o.json {
			return writeJSON(struct {
				Projects []projects.Item `json:"projects"`
			}{items})
		}
		for _, item := range items {
			renderProjectItem(item)
		}
		if len(items) == 0 {
			ui.Note("No projects registered. Use skillshare project add <root> --target T.")
		}
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var root string
	if o.action == "add" {
		root, err = projects.StoredKey(o.token, cwd, home)
	} else {
		root, err = projects.Find(cfg.Projects, o.token, cwd, home)
	}
	if err != nil {
		return err
	}

	if o.action == "show" {
		item, err := service.Show(root)
		if err != nil {
			return err
		}
		if o.json {
			return writeJSON(struct {
				Project projects.Item `json:"project"`
			}{item})
		}
		renderProjectItem(item)
		return nil
	}
	if o.action == "remove" && !o.dryRun && !o.yes {
		ok, err := ui.ConfirmAction("Remove project "+root+"?", false)
		if err != nil {
			return err
		}
		if !ok {
			ui.Cancelled("removed")
			return nil
		}
	}
	var result projects.Result
	var opErr error
	switch o.action {
	case "add", "edit":
		entry := cfg.Projects[root]
		if o.action == "add" {
			entry = config.ManagedProject{Skills: &config.ResourceTargetConfig{}}
		}
		entry, err = patchProject(entry, o)
		if err != nil {
			return err
		}
		result, opErr = service.Put(root, entry, projects.PutOptions{Create: o.action == "add", DryRun: o.dryRun})
		if opErr != nil {
			return opErr
		}
		if !o.dryRun && len(result.Affected) > 0 {
			logProject(cfg, "project", o.action, root, "ok", nil, start)
		}
		if o.sync {
			// Dry-run sync must use the unsaved candidate, including renamed targets.
			cfg = result.Expanded
			synced, syncErr := service.Sync(root, projects.ExecuteOptions{Kinds: []string{"skill", "agent"}, DryRun: o.dryRun})
			result.Sync = synced.Sync
			result.Warnings = append(result.Warnings, synced.Warnings...)
			opErr = projectSyncError(synced, syncErr)
			if !o.dryRun {
				logProject(cfg, "sync", "sync", root, statusFromErr(opErr), opErr, start)
			}
		}
	case "sync":
		result, opErr = service.Sync(root, projects.ExecuteOptions{Kinds: []string{"skill", "agent"}, DryRun: o.dryRun})
		opErr = projectSyncError(result, opErr)
		if !o.dryRun {
			logProject(cfg, "sync", "sync", root, statusFromErr(opErr), opErr, start)
		}
		if opErr != nil && result.Sync == nil {
			return opErr
		}
	case "remove":
		result, opErr = service.Remove(root, projects.RemoveOptions{DryRun: o.dryRun})
		changed := result.Detach != nil && (len(result.Detach.Removed) > 0 || len(result.Detach.CopiedBack) > 0)
		if opErr != nil && !changed {
			return opErr
		}
		if !o.dryRun && (opErr == nil || changed) {
			status := "ok"
			if opErr != nil {
				status = "partial"
			}
			logProject(cfg, "project", "remove", root, status, opErr, start)
		}
	}
	return renderProjectResult(o, result, opErr, start)
}

func patchProject(entry config.ManagedProject, o projectOptions) (config.ManagedProject, error) {
	entry.Targets = slices.Clone(entry.Targets)
	if values, ok := o.values["--target"]; ok {
		entry.Targets = nil
		for _, value := range values {
			for _, target := range strings.Split(value, ",") {
				target = strings.TrimSpace(target)
				if target != "" {
					entry.Targets = append(entry.Targets, target)
				}
			}
		}
	}
	if o.action == "add" && len(entry.Targets) == 0 {
		return entry, fmt.Errorf("project add requires at least one --target")
	}
	if values, ok := o.values["--skills-naming"]; ok && !config.IsValidTargetNaming(values[len(values)-1]) {
		return entry, fmt.Errorf("invalid skills naming %q (valid: %s)", values[len(values)-1], strings.Join(config.ValidTargetNamings, ", "))
	}
	if values, ok := o.values["--name"]; ok {
		entry.Name = values[len(values)-1]
	}
	for _, kind := range []string{"skills", "agents"} {
		slot := &entry.Skills
		if kind == "agents" {
			slot = &entry.Agents
		}
		if *slot != nil {
			copy := **slot
			copy.Include = slices.Clone(copy.Include)
			copy.Exclude = slices.Clone(copy.Exclude)
			*slot = &copy
		}
		if o.toggles["--no-"+kind] {
			*slot = nil
		}
		if o.toggles["--"+kind] {
			if *slot == nil {
				*slot = &config.ResourceTargetConfig{}
			}
			(*slot).SetEnabled(true)
		}
		for _, field := range []string{"mode", "naming", "include", "exclude"} {
			values, ok := o.values["--"+kind+"-"+field]
			if !ok {
				continue
			}
			if *slot == nil {
				*slot = &config.ResourceTargetConfig{}
			}
			switch field {
			case "mode":
				(*slot).Mode = values[len(values)-1]
			case "naming":
				(*slot).TargetNaming = values[len(values)-1]
			case "include", "exclude":
				var patterns []string
				for _, p := range values {
					if p == "" {
						patterns = nil
					} else {
						patterns = append(patterns, p)
					}
				}
				if field == "include" {
					(*slot).Include = patterns
				} else {
					(*slot).Exclude = patterns
				}
			}
		}
	}
	if entry.Agents != nil && (len(entry.Agents.Include) > 0 || len(entry.Agents.Exclude) > 0) {
		mode := entry.Agents.Mode
		if mode == "" {
			mode = "merge"
		}
		if mode == "symlink" {
			return entry, fmt.Errorf("agent include/exclude filters are ignored in symlink mode; use --agents-mode merge or copy")
		}
	}
	return entry, nil
}

func projectSyncError(result projects.Result, err error) error {
	if err != nil {
		return err
	}
	if result.Sync != nil && len(result.Sync.Failed) > 0 {
		return fmt.Errorf("sync failed for %d target resources", len(result.Sync.Failed))
	}
	return nil
}
func logProject(cfg *config.Config, command, action, root, status string, err error, start time.Time) {
	if action == "edit" {
		action = "update"
	}
	e := oplog.NewEntry(command, status, time.Since(start))
	e.Args = map[string]any{"action": action, "root": root, "scope": "cli"}
	if err != nil {
		e.Message = err.Error()
	}
	limit := config.DefaultLogMaxEntries
	if cfg.Log.MaxEntries != nil && *cfg.Log.MaxEntries >= 0 {
		limit = *cfg.Log.MaxEntries
	}
	// This command is always global. The selected config can live in a legacy
	// .../skillshare directory that oplog would otherwise mistake for a project.
	routingPath := filepath.Join(config.BaseDir(), "config.yaml")
	_ = oplog.WriteWithLimit(routingPath, oplog.OpsFile, e, limit)
}
