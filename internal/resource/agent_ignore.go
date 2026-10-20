package resource

import (
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

// AgentIgnoreStats describes ignore state from the same discovery snapshot used
// for sync. Ignored lists disabled agents by source-relative filename.
type AgentIgnoreStats struct {
	Root    string
	Ignored []string
}

// InspectAgentIgnore reads the ignore-file location and summarizes a supplied
// all-agents snapshot. It never discovers agents a second time.
func InspectAgentIgnore(source string, all []DiscoveredResource) AgentIgnoreStats {
	stats := AgentIgnoreStats{Ignored: []string{}}
	if source == "" {
		return stats
	}
	ignore := filepath.Join(utils.ResolveSymlink(source), ".agentignore")
	if _, err := os.Stat(ignore); err == nil {
		stats.Root = ignore
	}
	for _, agent := range all {
		if agent.Disabled {
			stats.Ignored = append(stats.Ignored, agent.RelPath)
		}
	}
	return stats
}
