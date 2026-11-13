//go:build !windows

package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// processGone treats a zombie as gone: a container's init may never reap an orphan.
func processGone(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return true
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return os.IsNotExist(err)
	}
	fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
	return len(fields) > 0 && fields[0] == "Z"
}

func TestLiveStdioTimeoutKillsProcessGroup(t *testing.T) {
	pids := filepath.Join(t.TempDir(), "pids")
	start := time.Now()
	report, err := checkService(t, liveConfig("hang", map[string]string{"MCP_LIVE_PIDS": pids})).Check(liveCheck(1500*time.Millisecond, tokenEnv))
	if err != nil {
		t.Fatal(err)
	}
	if _, f := liveOf(t, report); f.Level != "error" || !strings.Contains(f.Message, "no answer within 1.5s") {
		t.Fatalf("finding = %+v", f)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("the probe took %s", elapsed)
	}
	data, err := os.ReadFile(pids)
	if err != nil {
		t.Fatal(err)
	}
	var server, child int
	if _, err := fmt.Sscanf(string(data), "%d %d", &server, &child); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !(processGone(server) && processGone(child)) {
		if time.Now().After(deadline) {
			t.Fatalf("server %d or its child %d is still running", server, child)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
