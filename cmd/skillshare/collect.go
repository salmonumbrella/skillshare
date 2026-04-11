package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pterm/pterm"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

// collectJSONOutput is the JSON representation for collect --json output.
type collectJSONOutput struct {
	Pulled   []string          `json:"pulled"`
	Skipped  []string          `json:"skipped"`
	Failed   map[string]string `json:"failed"`
	DryRun   bool              `json:"dry_run"`
	Duration string            `json:"duration"`
}

// collectLocalSkills collects local skills from targets (non-symlinked)
func collectLocalSkills(targets map[string]config.TargetConfig, source, globalMode string) []sync.LocalSkillInfo {
	var allLocalSkills []sync.LocalSkillInfo
	for name, target := range targets {
		sc := target.SkillsConfig()
		mode := sync.EffectiveMode(sc.Mode)
		if sc.Mode == "" && globalMode != "" {
			mode = globalMode
		}
		skills, err := sync.FindLocalSkills(sc.Path, source, mode)
		if err != nil {
			ui.Warning("%s: %v", name, err)
			continue
		}
		for i := range skills {
			skills[i].TargetName = name
		}
		allLocalSkills = append(allLocalSkills, skills...)
	}
	return allLocalSkills
}

// displayLocalSkills shows the local skills found
func displayLocalSkills(skills []sync.LocalSkillInfo) {
	ui.Header(ui.WithModeLabel("Local skills found"))
	for _, skill := range skills {
		ui.ListItem("info", skill.Name, fmt.Sprintf("[%s] %s", skill.TargetName, skill.Path))
	}
}

func cmdCollect(args []string) error {
	if wantsHelp(args) {
		printCollectHelp()
		return nil
	}

	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	resources, rest, err := parseResourceFlags(rest, resourceFlagOptions{
		defaultSelection: resourceSelection{skills: true},
	})
	if err != nil {
		return err
	}

	if mode == modeProject {
		err := cmdCollectProject(rest, cwd, resources)
		logCollectOp(config.ProjectConfigPath(cwd), start, err)
		return err
	}

	dryRun := false
	force := false
	collectAll := false
	jsonOutput := false
	var targetName string

	for _, arg := range rest {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--force", "-f":
			force = true
		case "--all", "-a":
			collectAll = true
		case "--json":
			jsonOutput = true
		default:
			if targetName == "" && !strings.HasPrefix(arg, "-") {
				targetName = arg
			}
		}
	}

	// --json implies --force (skip confirmation prompts)
	if jsonOutput {
		force = true
	}

	if resources.onlyManaged() {
		if targetName != "" || collectAll {
			return fmt.Errorf("target selection is only supported when collecting skills")
		}
		if jsonOutput {
			result, collectErr := collectManagedResources("", resources, dryRun, force)
			logCollectOp(config.ConfigPath(), start, collectErr)
			return collectOutputJSON(result, dryRun, start, collectErr)
		}
		err := executeManagedCollect("", resources, dryRun, force)
		logCollectOp(config.ConfigPath(), start, err)
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		if jsonOutput {
			return writeJSONError(err)
		}
		return err
	}

	// Select targets to collect from
	targets, err := selectCollectTargets(cfg, targetName, collectAll)
	if err != nil {
		return err
	}
	if targets == nil {
		return nil // User needs to specify target
	}

	// Collect all local skills
	var sp *ui.Spinner
	if !jsonOutput {
		ui.Header(ui.WithModeLabel("Collect"))
		sp = ui.StartSpinner("Scanning for local skills...")
	}

	allLocalSkills := collectLocalSkills(targets, cfg.Source, cfg.Mode)

	if len(allLocalSkills) == 0 {
		if sp != nil {
			sp.Success("No local skills found")
		}
		if !resources.includesManaged() {
			if jsonOutput {
				return collectOutputJSON(nil, dryRun, start, nil)
			}
			return nil
		}
	} else if sp != nil {
		sp.Success(fmt.Sprintf("Found %d local skill(s)", len(allLocalSkills)))
		displayLocalSkills(allLocalSkills)
	}

	if dryRun {
		if jsonOutput {
			var result *sync.PullResult
			if len(allLocalSkills) > 0 {
				names := make([]string, len(allLocalSkills))
				for i, s := range allLocalSkills {
					names[i] = s.Name
				}
				result = &sync.PullResult{Pulled: names, Failed: make(map[string]error)}
			}
			if resources.includesManaged() {
				managedResult, managedErr := collectManagedResources("", resources, true, force)
				result = mergePullResults(result, managedResult)
				logCollectOp(config.ConfigPath(), start, managedErr)
				return collectOutputJSON(result, true, start, managedErr)
			}
			logCollectOp(config.ConfigPath(), start, nil)
			return collectOutputJSON(result, true, start, nil)
		}
		if resources.includesManaged() {
			err := executeManagedCollect("", resources, true, force)
			logCollectOp(config.ConfigPath(), start, err)
			return err
		}
		ui.Info("Dry run - no changes made")
		return nil
	}

	// Confirm unless --force (JSON implies force)
	if !force && len(allLocalSkills) > 0 {
		if !confirmCollect() {
			ui.Info("Cancelled")
			return nil
		}
	}

	if jsonOutput {
		var result *sync.PullResult
		var collectErr error
		if len(allLocalSkills) > 0 {
			result, collectErr = sync.PullSkills(allLocalSkills, cfg.Source, sync.PullOptions{
				DryRun: dryRun,
				Force:  force,
			})
			collectErr = combineCollectErrors(collectErr, collectResultError(result))
		}
		var managedErr error
		if resources.includesManaged() {
			var managedResult *sync.PullResult
			managedResult, managedErr = collectManagedResources("", resources, dryRun, force)
			result = mergePullResults(result, managedResult)
		}
		err = combineCollectErrors(collectErr, managedErr)
		logCollectOp(config.ConfigPath(), start, err)
		return collectOutputJSON(result, dryRun, start, err)
	}

	var collectErr error
	if len(allLocalSkills) > 0 {
		collectErr = executeCollect(allLocalSkills, cfg.Source, dryRun, force)
	}
	var managedErr error
	if resources.includesManaged() {
		managedErr = executeManagedCollect("", resources, dryRun, force)
	}
	err = combineCollectErrors(collectErr, managedErr)
	logCollectOp(config.ConfigPath(), start, err)
	return err
}

func logCollectOp(cfgPath string, start time.Time, cmdErr error) {
	e := oplog.NewEntry("collect", statusFromErr(cmdErr), time.Since(start))
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func selectCollectTargets(cfg *config.Config, targetName string, collectAll bool) (map[string]config.TargetConfig, error) {
	if targetName != "" {
		if t, exists := cfg.Targets[targetName]; exists {
			return map[string]config.TargetConfig{targetName: t}, nil
		}
		return nil, fmt.Errorf("target '%s' not found", targetName)
	}

	if collectAll || len(cfg.Targets) == 1 {
		return cfg.Targets, nil
	}

	// If no target specified and multiple targets exist, ask or require --all
	ui.Warning("Multiple targets found. Specify a target name or use --all")
	fmt.Println("  Available targets:")
	for name := range cfg.Targets {
		fmt.Printf("    - %s\n", name)
	}
	return nil, nil
}

func confirmCollect() bool {
	fmt.Println()
	fmt.Print("Collect these skills to source? [y/N]: ")
	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}

func executeCollect(skills []sync.LocalSkillInfo, source string, dryRun, force bool) error {
	ui.Header(ui.WithModeLabel("Collecting skills"))
	result, err := sync.PullSkills(skills, source, sync.PullOptions{
		DryRun: dryRun,
		Force:  force,
	})
	if err != nil {
		return err
	}

	// Display results
	for _, name := range result.Pulled {
		ui.StepDone(name, "copied to source")
	}
	for _, name := range result.Skipped {
		ui.StepSkip(name, "already exists in source, use --force to overwrite")
	}
	for name, err := range result.Failed {
		ui.StepFail(name, err.Error())
	}

	ui.OperationSummary("Collect", 0,
		ui.Metric{Label: "collected", Count: len(result.Pulled), HighlightColor: pterm.Green},
		ui.Metric{Label: "skipped", Count: len(result.Skipped), HighlightColor: pterm.Yellow},
		ui.Metric{Label: "failed", Count: len(result.Failed), HighlightColor: pterm.Red},
	)

	if len(result.Pulled) > 0 {
		showCollectNextSteps(source)
	}

	return collectResultError(result)
}

// collectOutputJSON converts a collect result to JSON and writes to stdout.
func collectOutputJSON(result *sync.PullResult, dryRun bool, start time.Time, collectErr error) error {
	output := collectJSONOutput{
		DryRun:   dryRun,
		Duration: formatDuration(start),
	}
	output.Failed = make(map[string]string)
	if result != nil {
		output.Pulled = result.Pulled
		output.Skipped = result.Skipped
		for k, v := range result.Failed {
			output.Failed[k] = v.Error()
		}
	}
	return writeJSONResult(&output, collectErr)
}

func showCollectNextSteps(source string) {
	fmt.Println()
	if ui.ModeLabel == "project" {
		ui.Info("Run 'skillshare sync -p' to distribute to all targets")
		return
	}
	ui.Info("Run 'skillshare sync' to distribute to all targets")

	// Check if source has git
	if strings.TrimSpace(source) == "" {
		return
	}
	gitDir := filepath.Join(source, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		ui.Info("Commit changes: cd %s && git add . && git commit", source)
	}
}

func mergePullResults(left, right *sync.PullResult) *sync.PullResult {
	switch {
	case left == nil:
		return right
	case right == nil:
		return left
	}

	merged := &sync.PullResult{
		Pulled:  append(append([]string{}, left.Pulled...), right.Pulled...),
		Skipped: append(append([]string{}, left.Skipped...), right.Skipped...),
		Failed:  make(map[string]error, len(left.Failed)+len(right.Failed)),
	}
	for name, err := range left.Failed {
		merged.Failed[name] = err
	}
	for name, err := range right.Failed {
		merged.Failed[name] = err
	}
	return merged
}

func collectResultError(result *sync.PullResult) error {
	if result == nil || len(result.Failed) == 0 {
		return nil
	}
	return fmt.Errorf("some skills failed to collect")
}

func combineCollectErrors(errs ...error) error {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		messages = append(messages, err.Error())
	}
	if len(messages) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(messages, "; "))
}

func printCollectHelp() {
	fmt.Println(`Usage: skillshare collect [target] [options]

Collect local skills from target(s) to source directory.

Arguments:
  [target]          Target name to collect from (optional)

Options:
  --all, -a         Collect from all targets
  --dry-run, -n     Preview changes without applying
  --force, -f       Skip confirmation prompts
  --json            Output results as JSON
  --project, -p     Use project-level config
  --global, -g      Use global config
  --help, -h        Show this help

Examples:
  skillshare collect claude      Collect from Claude target
  skillshare collect --all       Collect from all targets
  skillshare collect --dry-run   Preview what would be collected`)
}
