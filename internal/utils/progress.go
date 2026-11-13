package utils

import (
	"io"
	"time"
)

// progressInterval throttles callbacks so spinners don't flicker.
const progressInterval = 500 * time.Millisecond

// ProgressFunc receives bytes read so far and the expected total (-1 if unknown).
type ProgressFunc func(read, total int64)

// NewProgressReader wraps r and reports progress to fn, at most once per
// progressInterval plus a final report at EOF. A nil fn returns r unchanged.
func NewProgressReader(r io.Reader, total int64, fn ProgressFunc) io.Reader {
	if fn == nil {
		return r
	}
	return &progressReader{r: r, total: total, fn: fn}
}

type progressReader struct {
	r        io.Reader
	total    int64
	read     int64
	fn       ProgressFunc
	lastSent time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if err == io.EOF || time.Since(p.lastSent) >= progressInterval {
		p.lastSent = time.Now()
		p.fn(p.read, p.total)
	}
	return n, err
}
