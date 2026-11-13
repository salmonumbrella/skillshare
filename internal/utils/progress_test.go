package utils

import (
	"io"
	"strings"
	"testing"
)

func TestProgressReader_FinalReportIsTotalBytes(t *testing.T) {
	data := strings.Repeat("x", 100_000)
	var lastRead, lastTotal int64
	r := NewProgressReader(strings.NewReader(data), int64(len(data)), func(read, total int64) {
		lastRead, lastTotal = read, total
	})

	if _, err := io.Copy(io.Discard, r); err != nil {
		t.Fatal(err)
	}
	if lastRead != int64(len(data)) || lastTotal != int64(len(data)) {
		t.Errorf("final report = %d/%d, want %d/%d", lastRead, lastTotal, len(data), len(data))
	}
}
