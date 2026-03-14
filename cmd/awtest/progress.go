package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"golang.org/x/term"
)

// isTerminal reports whether the given file descriptor is a terminal.
func isTerminal(fd int) bool {
	return term.IsTerminal(fd)
}

// progressReporter displays in-place scan progress on stderr.
// A nil *progressReporter is safe to call methods on (no-op).
type progressReporter struct {
	completed int64        // atomic counter
	total     int          // total services to scan
	done      chan struct{} // signal to stop ticker
	writer    *os.File     // stderr
}

// newProgressReporter returns a progress reporter that writes to stderr.
// Returns nil (suppressing progress) when quiet is true or stderr is not a TTY.
func newProgressReporter(total int, quiet bool) *progressReporter {
	if quiet || !isTerminal(int(os.Stderr.Fd())) {
		return nil
	}
	return &progressReporter{
		total:  total,
		done:   make(chan struct{}),
		writer: os.Stderr,
	}
}

// Increment atomically increments the completed service count.
func (p *progressReporter) Increment() {
	if p == nil {
		return
	}
	atomic.AddInt64(&p.completed, 1)
}

// Start launches a ticker goroutine that writes in-place progress to stderr at 2 Hz.
func (p *progressReporter) Start() {
	if p == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				count := atomic.LoadInt64(&p.completed)
				fmt.Fprintf(p.writer, "\r%-50s", fmt.Sprintf("Scanning... %d/%d services complete", count, p.total))
			case <-p.done:
				return
			}
		}
	}()
}

// Stop stops the ticker goroutine and clears the progress line from stderr.
func (p *progressReporter) Stop() {
	if p == nil {
		return
	}
	close(p.done)
	// Clear progress line: overwrite with spaces, then reset cursor
	fmt.Fprintf(p.writer, "\r%-50s\r", "")
}
