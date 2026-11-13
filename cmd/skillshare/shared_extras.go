package main

import (
	"errors"
	"fmt"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func sharedDetachEntries(results []sync.DetachResult, jsonOutput bool, totals *extrasSyncTotals) []syncExtrasJSONEntry {
	var entries []syncExtrasJSONEntry
	for _, result := range results {
		target := syncExtrasJSONTarget{Path: result.Target, Pruned: result.Removed, Warnings: []string{}}
		totals.detached += result.Removed
		totals.kept += len(result.Kept)
		totals.detachErrors += len(result.Errors)
		totals.pruned += result.Removed
		for _, path := range result.Kept {
			target.Warnings = append(target.Warnings, "kept: edited by you: "+path)
		}
		for _, err := range result.Errors {
			totals.errors++
			if target.Error != "" {
				target.Error += "; "
			}
			target.Error += err
		}
		if !jsonOutput {
			if result.Removed > 0 {
				ui.Note(fmt.Sprintf("Detached %s: %d placement(s)", result.Name, result.Removed))
			}
			for _, warning := range target.Warnings {
				ui.Warning("%s", warning)
			}
			for _, err := range result.Errors {
				ui.Warning("%s", err)
			}
		}
		entries = append(entries, syncExtrasJSONEntry{Name: result.Name, Targets: []syncExtrasJSONTarget{target}})
	}
	return entries
}

func runGlobalExtrasSyncEntries(cfg *config.Config, dryRun, force bool, agentPaths map[string]bool) ([]syncExtrasJSONEntry, error) {
	session, err := sync.BeginSharedExtras(cfg, dryRun)
	if err != nil {
		return nil, err
	}
	totals := extrasSyncTotals{}
	entries := sharedDetachEntries(session.Detach(config.ExpandPath), true, &totals)
	applied, applyErr := runExtrasSyncEntries(cfg.Extras, func(extra config.ExtraConfig) string {
		return config.ResolveExtrasSourceDir(extra, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource())
	}, dryRun, force, "", agentPaths, session)
	entries = append(entries, applied...)
	var detachErr error
	if totals.errors > 0 {
		detachErr = fmt.Errorf("%d shared extras detach error(s)", totals.errors)
	}
	return entries, errors.Join(detachErr, applyErr, session.Save())
}

func sharedStatusPayload(cfg *config.Config) *config.SharedStatus {
	if !cfg.SharedEnabled() {
		return nil
	}
	status := cfg.SharedStatus()
	return &status
}

func printSharedStatus(cfg *config.Config) {
	if !cfg.SharedEnabled() {
		return
	}
	status := cfg.SharedStatus()
	text := status.Path
	if !status.Usable {
		text += " · unavailable · " + string(status.Reason)
	} else {
		count, skipped, overrides := 0, 0, 0
		for _, entry := range status.Entries {
			switch entry.Origin {
			case "shared":
				count++
			case "skipped":
				count++
				skipped++
			case "overrides shared":
				count++
				overrides++
			}
		}
		text += fmt.Sprintf(" · %d extras (%d skipped, %d overrides)", count, skipped, overrides)
	}
	ui.Row(ui.MarkNone, "Shared", text, ui.RowWidth("Shared"))
}
func checkSharedFile(cfg *config.Config, result *doctorResult) {
	if !cfg.SharedEnabled() {
		return
	}
	status := cfg.SharedStatus()
	if err := cfg.CheckSharedApply(); err != nil {
		result.addError()
		result.addCheck("shared_file", checkError, err.Error(), nil)
		ui.Warning("Shared: %v", err)
		return
	}
	warnings := append([]string(nil), status.Warnings...)
	for _, extra := range cfg.Extras {
		for _, target := range extra.Targets {
			if target.Extension != "" {
				if _, err := resolveExtension(target.Extension, globalExtensionsDir()); err != nil {
					warnings = append(warnings, "extras "+extra.Name+": extension unavailable")
				}
			}
		}
	}
	if len(warnings) > 0 {
		result.addWarning()
		result.addCheck("shared_file", checkWarning, "Shared settings are usable with portability warnings", warnings)
		ui.Warning("Shared: %s", status.Path)
		return
	}
	result.addCheck("shared_file", checkPass, "Shared settings are usable", nil)
}
