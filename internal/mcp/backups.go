package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// keepBackups bounds the backups kept per Agent file; every write adds one.
const keepBackups = 20

// BackupInfo exposes metadata only, never the backed-up connection values.
type BackupInfo struct {
	ID      string         `json:"id"`
	Target  string         `json:"target"`
	Path    string         `json:"path"`
	Time    string         `json:"time"` // RFC3339, from the ID's nanosecond prefix
	Servers []BackupServer `json:"servers"`
}

// BackupServer names one server entry the backed-up write changed.
type BackupServer struct {
	Name   string `json:"name"`
	Change string `json:"change"` // added, changed, or removed
}

// backupServers lists the entries a write changed, from the stored before and
// after values, sorted by name.
func backupServers(record backupRecord) []BackupServer {
	names := make([]string, 0, len(record.After))
	for name := range record.After {
		names = append(names, name)
	}
	sort.Strings(names)
	servers := make([]BackupServer, 0, len(names))
	for _, name := range names {
		change := "changed"
		switch {
		case record.Before[name] == nil && record.After[name] != nil:
			change = "added"
		case record.Before[name] != nil && record.After[name] == nil:
			change = "removed"
		}
		servers = append(servers, BackupServer{Name: name, Change: change})
	}
	return servers
}

// backupTime reads the nanosecond time that starts a backup ID.
func backupTime(id string) string {
	stamp, _, _ := strings.Cut(id, "-")
	n, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return ""
	}
	return time.Unix(0, n).Format(time.RFC3339)
}

func (s *Service) Backups() ([]BackupInfo, error) {
	result := []BackupInfo{}
	owner, err := filepath.Abs(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	records, err := s.backupRecords()
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.Owner == owner {
			result = append(result, BackupInfo{ID: record.ID, Target: shownTarget(record.Target), Path: record.Path, Time: backupTime(record.ID), Servers: backupServers(record)})
		}
	}
	return result, nil
}

// backupRecords returns backups newest first. Unreadable files are skipped so
// one damaged backup cannot hide the others.
func (s *Service) backupRecords() ([]backupRecord, error) {
	dir := filepath.Join(s.StateDir, "mcp", "backups")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []backupRecord
	// IDs start with a 19-digit nanosecond time, so name order is time order.
	for i := len(entries) - 1; i >= 0; i-- {
		name := entries[i].Name()
		data, _, _, err := safeRead(filepath.Join(dir, name))
		var record backupRecord
		if err != nil || json.Unmarshal(data, &record) != nil || !backupID.MatchString(record.ID) || record.ID+".json" != name {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

// backupSuffix ends every backup ID of one Agent file, so pruning can select
// them by name without decoding each backup.
func backupSuffix(path string) string { return digest([]byte(path))[:8] }

// pruneBackups keeps the newest backups of one Agent file. It is best effort:
// a backup that fails to delete only costs disk space.
func (s *Service) pruneBackups(path string) {
	dir := filepath.Join(s.StateDir, "mcp", "backups")
	entries, _ := os.ReadDir(dir)
	suffix := "-" + backupSuffix(path) + ".json"
	kept := 0
	// IDs start with a 19-digit nanosecond time, so name order is time order.
	for i := len(entries) - 1; i >= 0; i-- {
		if name := entries[i].Name(); strings.HasSuffix(name, suffix) {
			if kept++; kept > keepBackups {
				_ = os.Remove(filepath.Join(dir, name))
			}
		}
	}
}
