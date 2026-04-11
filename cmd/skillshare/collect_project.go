package main

import (
	"fmt"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

func cmdCollectProject(args []string, root string, resources resourceSelection) error {
	dryRun := false
	force := false
	collectAll := false
	var targetName string

	for _, arg := range args {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--force", "-f":
			force = true
		case "--all", "-a":
			collectAll = true
		default:
			if targetName == "" && !strings.HasPrefix(arg, "-") {
				targetName = arg
			}
		}
	}

	if resources.onlyManaged() {
		if targetName != "" || collectAll {
			return fmt.Errorf("target selection is only supported when collecting skills")
		}
		return executeManagedCollect(root, resources, dryRun, force)
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return err
	}

	targets, err := selectCollectProjectTargets(runtime, targetName, collectAll)
	if err != nil {
		return err
	}
	if targets == nil {
		return nil
	}

	ui.Header(ui.WithModeLabel("Collect"))
	sp := ui.StartSpinner("Scanning for local skills...")
	allLocalSkills := collectLocalSkills(targets, runtime.sourcePath, "")
	if len(allLocalSkills) == 0 {
		sp.Success("No local skills found")
		if resources.includesManaged() {
			if dryRun {
				return executeManagedCollect(root, resources, true, force)
			}
			return executeManagedCollect(root, resources, false, force)
		}
		return nil
	}
	sp.Success(fmt.Sprintf("Found %d local skill(s)", len(allLocalSkills)))

	displayLocalSkills(allLocalSkills)

	if dryRun {
		if resources.includesManaged() {
			return executeManagedCollect(root, resources, true, force)
		}
		ui.Info("Dry run - no changes made")
		return nil
	}

	if !force && len(allLocalSkills) > 0 {
		if !confirmCollect() {
			ui.Info("Cancelled")
			return nil
		}
	}

	var collectErr error
	if len(allLocalSkills) > 0 {
		collectErr = executeCollect(allLocalSkills, runtime.sourcePath, dryRun, force)
	}
	var managedErr error
	if resources.includesManaged() {
		managedErr = executeManagedCollect(root, resources, dryRun, force)
	}
	return combineCollectErrors(collectErr, managedErr)
}

func selectCollectProjectTargets(runtime *projectRuntime, targetName string, collectAll bool) (map[string]config.TargetConfig, error) {
	if targetName != "" {
		if t, ok := runtime.targets[targetName]; ok {
			return map[string]config.TargetConfig{targetName: t}, nil
		}
		return nil, fmt.Errorf("target '%s' not found in project config", targetName)
	}

	if collectAll || len(runtime.targets) == 1 {
		return runtime.targets, nil
	}

	ui.Warning("Multiple targets found. Specify a target name or use --all")
	fmt.Println("  Available targets:")
	for _, entry := range runtime.config.Targets {
		fmt.Printf("    - %s\n", entry.Name)
	}
	return nil, nil
}
