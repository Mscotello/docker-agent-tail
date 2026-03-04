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
	maxNameLen     int
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
// maxNameLen is the longest container name for fixed-width formatting.
func NewLogWriter(sessionDir string, maxNameLen int) (*LogWriter, error) {
	// Create combined.log
	combinedPath := filepath.Join(sessionDir, "combined.log")
	combinedFile, err := os.Create(combinedPath)
	if err != nil {
		return nil, fmt.Errorf("creating combined log: %w", err)
	}

	w := &LogWriter{
		sessionDir:     sessionDir,
		maxNameLen:     maxNameLen,
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

// writeLine writes to per-container file and combined log.
func (w *LogWriter) writeLine(line docker.LogLine) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Per-container format
	perContainerLine := formatLogLine(line, false)

	// Write to per-container file
	containerFile, err := w.getOrCreateFile(line.ContainerName)
	if err == nil {
		_, _ = containerFile.WriteString(perContainerLine)
	}

	// Combined format with fixed-width container name
	combinedLine := formatCombinedLine(line, w.maxNameLen)
	_, _ = w.combinedFile.WriteString(combinedLine)
}

// getOrCreateFile returns the file handle for a container, creating if needed.
func (w *LogWriter) getOrCreateFile(containerName string) (*os.File, error) {
	if f, ok := w.containerFiles[containerName]; ok {
		return f, nil
	}

	path := filepath.Join(w.sessionDir, containerName+".log")
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

// formatLogLine formats a log line for per-container output.
func formatLogLine(line docker.LogLine, combined bool) string {
	ts := line.Timestamp.Format("2006-01-02T15:04:05.999Z07:00")
	return fmt.Sprintf("[%s] [%s] %s\n", ts, line.Stream, line.Content)
}

// formatCombinedLine formats a log line for combined output with fixed-width container name.
func formatCombinedLine(line docker.LogLine, maxNameLen int) string {
	ts := line.Timestamp.Format("2006-01-02T15:04:05.999Z07:00")
	containerName := line.ContainerName
	if len(containerName) < maxNameLen {
		containerName = containerName + " " + string(make([]byte, maxNameLen-len(line.ContainerName)))
	}
	return fmt.Sprintf("[%s] [%-*s] [%s] %s\n", ts, maxNameLen, line.ContainerName, line.Stream, line.Content)
}
