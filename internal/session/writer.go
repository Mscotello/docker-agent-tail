// Package session provides session directory and symlink management.
package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Mscotello/docker-agent-tail/internal/docker"
)

// LogWriter writes logs to per-container files and a combined log file.
type LogWriter struct {
	sessionDir     string
	combinedFile   *os.File
	containerFiles map[string]*os.File
	flushTicker    *time.Ticker
	done           chan struct{}
	queue          chan logEntry
	mu             sync.Mutex
}

// logEntry is an internal queue entry for thread-safe writing.
type logEntry struct {
	line docker.LogLine
	file string // "combined" or container name
}

// NewLogWriter creates a new log writer for a session.
func NewLogWriter(sessionDir string) (*LogWriter, error) {
	combinedPath := filepath.Join(sessionDir, "combined.jsonl")
	combinedFile, err := os.Create(combinedPath)
	if err != nil {
		return nil, fmt.Errorf("creating combined log: %w", err)
	}

	w := &LogWriter{
		sessionDir:     sessionDir,
		combinedFile:   combinedFile,
		containerFiles: make(map[string]*os.File),
		flushTicker:    time.NewTicker(100 * time.Millisecond),
		done:           make(chan struct{}),
		queue:          make(chan logEntry, 1000),
	}

	// Start background flusher
	go w.flusher()

	return w, nil
}

// Write queues a log line for writing.
func (w *LogWriter) Write(line docker.LogLine) {
	w.queue <- logEntry{line: line}
}

// flusher processes queued log entries and periodically flushes.
func (w *LogWriter) flusher() {
	for {
		select {
		case entry := <-w.queue:
			w.writeLine(entry.line)
		case <-w.flushTicker.C:
			w.flush()
		case <-w.done:
			// Drain remaining queue
			for {
				select {
				case entry := <-w.queue:
					w.writeLine(entry.line)
				default:
					w.flush()
					return
				}
			}
		}
	}
}

// writeLine writes to per-container file and combined log in JSONL format.
func (w *LogWriter) writeLine(line docker.LogLine) {
	w.mu.Lock()
	defer w.mu.Unlock()

	jsonl := FormatJSONL(line)

	// Write to per-container file
	containerFile, err := w.getOrCreateFile(line.ContainerName)
	if err == nil {
		_, _ = containerFile.Write(jsonl)
	}

	// Write to combined file
	_, _ = w.combinedFile.Write(jsonl)
}

// getOrCreateFile returns the file handle for a container, creating if needed.
func (w *LogWriter) getOrCreateFile(containerName string) (*os.File, error) {
	if f, ok := w.containerFiles[containerName]; ok {
		return f, nil
	}

	path := filepath.Join(w.sessionDir, containerName+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening container log: %w", err)
	}

	w.containerFiles[containerName] = f
	return f, nil
}

// flush syncs all open files.
func (w *LogWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	_ = w.combinedFile.Sync()
	for _, f := range w.containerFiles {
		_ = f.Sync()
	}
}

// Close flushes and closes all files.
func (w *LogWriter) Close() error {
	w.flushTicker.Stop()
	close(w.done)
	time.Sleep(100 * time.Millisecond) // Allow flusher goroutine to finish

	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.combinedFile.Close(); err != nil {
		return fmt.Errorf("closing combined log: %w", err)
	}

	for _, f := range w.containerFiles {
		_ = f.Close()
	}

	return nil
}

