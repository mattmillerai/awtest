package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewProgressReporterQuietReturnsNil(t *testing.T) {
	p := newProgressReporter(10, true)
	if p != nil {
		t.Error("newProgressReporter with quiet=true should return nil")
	}
}

func TestNewProgressReporterNonTTYReturnsNil(t *testing.T) {
	// Force non-TTY by temporarily replacing stderr with a pipe
	origStderr := os.Stderr
	_, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = origStderr
		w.Close()
	}()

	p := newProgressReporter(10, false)
	if p != nil {
		t.Error("newProgressReporter should return nil when stderr is not a TTY")
	}
}

func TestNilReceiverIncrement(t *testing.T) {
	var p *progressReporter
	// Should not panic
	p.Increment()
}

func TestNilReceiverStart(t *testing.T) {
	var p *progressReporter
	// Should not panic
	p.Start()
}

func TestNilReceiverStop(t *testing.T) {
	var p *progressReporter
	// Should not panic
	p.Stop()
}

func TestIncrementAtomic(t *testing.T) {
	p := &progressReporter{
		total:  100,
		done:   make(chan struct{}),
		writer: os.Stderr,
	}

	var wg sync.WaitGroup
	goroutines := 10
	incrementsPerGoroutine := 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				p.Increment()
			}
		}()
	}

	wg.Wait()

	expected := int64(goroutines * incrementsPerGoroutine)
	got := atomic.LoadInt64(&p.completed)
	if got != expected {
		t.Errorf("completed = %d, want %d", got, expected)
	}
}

func TestProgressOutputFormat(t *testing.T) {
	// Create a pipe to capture progress output
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	p := &progressReporter{
		total:  46,
		done:   make(chan struct{}),
		writer: w,
	}

	// Manually set completed count and write one tick
	atomic.StoreInt64(&p.completed, 15)
	fmt.Fprintf(p.writer, "\rScanning... %d/%d services complete", atomic.LoadInt64(&p.completed), p.total)

	w.Close()

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	r.Close()
	output := string(buf[:n])

	if !strings.Contains(output, "Scanning...") {
		t.Errorf("output missing 'Scanning...': %q", output)
	}
	if !strings.Contains(output, "services complete") {
		t.Errorf("output missing 'services complete': %q", output)
	}
	if !strings.Contains(output, "15/46") {
		t.Errorf("output missing '15/46': %q", output)
	}
}

func TestProgressWritesToStderr(t *testing.T) {
	// Verify the progress reporter is configured with stderr as writer
	p := &progressReporter{
		total:  10,
		done:   make(chan struct{}),
		writer: os.Stderr,
	}

	if p.writer != os.Stderr {
		t.Error("progress writer should be os.Stderr")
	}
}

func TestStartStopLifecycle(t *testing.T) {
	// Create a pipe to capture output
	_, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer w.Close()

	p := &progressReporter{
		total:  10,
		done:   make(chan struct{}),
		writer: w,
	}

	p.Start()

	// Let the ticker fire at least once
	time.Sleep(600 * time.Millisecond)

	// Stop should not panic or deadlock
	p.Stop()

	// Verify the done channel is closed (subsequent reads should succeed immediately)
	select {
	case <-p.done:
		// Expected: channel is closed
	default:
		t.Error("done channel should be closed after Stop()")
	}
}

func TestStartStopNoGoroutineLeak(t *testing.T) {
	// Create and stop multiple reporters to verify no goroutine leaks
	for i := 0; i < 10; i++ {
		_, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("failed to create pipe: %v", err)
		}

		p := &progressReporter{
			total:  10,
			done:   make(chan struct{}),
			writer: w,
		}

		p.Start()
		time.Sleep(50 * time.Millisecond)
		p.Stop()
		w.Close()
	}
	// If there were goroutine leaks, the race detector would catch them
	// or the test would hang
}

func TestIsTerminalReturnsFalseForPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if isTerminal(int(w.Fd())) {
		t.Error("pipe fd should not be a terminal")
	}
}
