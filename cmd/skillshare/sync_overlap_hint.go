package main

import (
	"fmt"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
)

// printSyncOverlapHint warns when enabled targets have overlapping skill paths
// (same primary path or cross-runtime discovery via also_scans). Targets that
// share a folder with different settings undo each other's sync, so they get
// a specific line and the command to keep one; any other overlap keeps the
// brief line pointing at `doctor`.
func printSyncOverlapHint(targets map[string]config.TargetConfig, isProject, jsonOutput bool, defaultMode string, discovered []sync.DiscoveredSkill) {
	if jsonOutput {
		return
	}
	conflicts, rest := sync.TargetFolderConflicts(targets, isProject, defaultMode, sync.HarmlessOverlap(targets, defaultMode, discovered))
	printSyncFolderConflicts(conflicts, rest, isProject)
}

// printSyncFolderConflicts renders prepared diagnostics without rediscovering targets.
func printSyncFolderConflicts(conflicts []config.SkillsFolderConflict, rest int, isProject bool) {
	for _, c := range conflicts {
		ui.Warning("%s sync skills to %s with different settings, so each sync undoes the other", joinAnd(c.Targets), shortenPath(c.Path))
		for _, name := range c.Stop {
			fmt.Println(ui.DimText("  keep one: " + skillsOffCommand(name, isProject)))
		}
	}

	if rest > 0 {
		ui.Warning("%s share skill folders %s %s", plural(rest, "target"), theme.Dim().Render("— see"), theme.Accent().Render("skillshare doctor"))
	}
}

// skillsOffCommand is the command that stops syncing skills to target name.
func skillsOffCommand(name string, isProject bool) string {
	if project, _, ok := config.SplitProjectTarget(name); ok && !isProject {
		return "skillshare project edit --no-skills -- '" + strings.ReplaceAll(project, "'", "'\\''") + "'"
	}
	cmd := "skillshare target " + name + " --skills=false"
	if isProject {
		cmd += " -p"
	}
	return cmd
}

// joinAnd joins names as "a and b" or "a, b and c".
func joinAnd(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
