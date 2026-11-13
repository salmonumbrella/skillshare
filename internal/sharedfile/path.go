package sharedfile

import (
	"os"
	"path/filepath"
	"strings"
)

func windowsAbsolute(p string) bool {
	return len(p) > 2 && p[1] == ':' && (p[2] == '/' || p[2] == '\\') || strings.HasPrefix(p, `\\`)
}
func ResolvePath(dir, p string) string {
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, filepath.FromSlash(strings.ReplaceAll(strings.TrimLeft(p[1:], `/\`), `\`, "/")))
		}
	}
	if filepath.IsAbs(p) || windowsAbsolute(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(p, `\`, "/"))))
}
func IsPortablePath(p string) bool { return !filepath.IsAbs(p) && !windowsAbsolute(p) }
