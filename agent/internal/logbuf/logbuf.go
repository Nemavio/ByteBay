package logbuf

import (
	"io"
	"strings"
	"sync"
	"time"
)

const maxLines = 800

type Entry struct {
	Time time.Time `json:"time"`
	Line string    `json:"line"`
}

var (
	mu   sync.Mutex
	lines []Entry
)

func Append(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	lines = append(lines, Entry{Time: time.Now().UTC(), Line: line})
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
}

type writer struct{}

func (writer) Write(p []byte) (int, error) {
	Append(string(p))
	return len(p), nil
}

func Writer() io.Writer { return writer{} }

func Since(since time.Time) []Entry {
	mu.Lock()
	defer mu.Unlock()
	if since.IsZero() {
		out := make([]Entry, len(lines))
		copy(out, lines)
		return out
	}
	var out []Entry
	for _, e := range lines {
		if e.Time.After(since) {
			out = append(out, e)
		}
	}
	return out
}
