// Package docker provides Docker SDK client operations.
package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// LogLine represents a single log entry from a container.
type LogLine struct {
	Timestamp     time.Time
	Stream        string // "stdout" or "stderr"
	Content       string
	ContainerName string
}

// StreamLogsOpts configures log streaming.
type StreamLogsOpts struct {
	Follow        bool
	Since         time.Time
	ContainerName string
}

// StreamLogs streams logs from a container via a channel.
// It demultiplexes stdout/stderr using stdcopy for non-TTY containers.
func StreamLogs(ctx context.Context, c DockerClient, containerID string, opts StreamLogsOpts) (<-chan LogLine, <-chan error) {
	logCh := make(chan LogLine)
	errCh := make(chan error, 1)

	go func() {
		defer close(logCh)

		// Get container inspect info to check for TTY
		resp, err := c.ContainerInspect(ctx, containerID)
		if err != nil {
			errCh <- fmt.Errorf("inspecting container: %w", err)
			return
		}

		logOpts := container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Timestamps: true,
			Follow:     opts.Follow,
		}
		if !opts.Since.IsZero() {
			logOpts.Since = opts.Since.Format(time.RFC3339)
		}

		logs, err := c.ContainerLogs(ctx, containerID, logOpts)
		if err != nil {
			errCh <- fmt.Errorf("getting container logs: %w", err)
			return
		}
		defer logs.Close()

		if resp.Config.Tty {
			// TTY containers send raw stream — read as stdout
			readStream(ctx, logs, logCh, errCh, opts.ContainerName, "stdout")
		} else {
			// Demux stdout/stderr via stdcopy
			outReader, outWriter := io.Pipe()
			errReader, errWriter := io.Pipe()

			go func() {
				if _, err := stdcopy.StdCopy(outWriter, errWriter, logs); err != nil {
					errCh <- fmt.Errorf("demuxing logs: %w", err)
				}
				outWriter.Close()
				errWriter.Close()
			}()

			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				readStream(ctx, outReader, logCh, errCh, opts.ContainerName, "stdout")
			}()
			go func() {
				defer wg.Done()
				readStream(ctx, errReader, logCh, errCh, opts.ContainerName, "stderr")
			}()
			wg.Wait()
		}
	}()

	return logCh, errCh
}

// readStream scans lines from a reader and sends parsed LogLines to logCh.
func readStream(ctx context.Context, reader io.Reader, logCh chan<- LogLine, errCh chan<- error, containerName, stream string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		logLine, err := parseLogLine(line, containerName, stream)
		if err != nil {
			continue
		}
		select {
		case logCh <- logLine:
		case <-ctx.Done():
			return
		}
	}
	if err := scanner.Err(); err != nil {
		select {
		case errCh <- fmt.Errorf("reading %s logs: %w", stream, err):
		default:
		}
	}
}

// parseLogLine parses a Docker log line with timestamp.
// Format: "2006-01-02T15:04:05.999999999Z content"
func parseLogLine(line, containerName, stream string) (LogLine, error) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 1 {
		return LogLine{}, fmt.Errorf("invalid log line")
	}

	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return LogLine{}, fmt.Errorf("parsing timestamp: %w", err)
	}

	content := ""
	if len(parts) > 1 {
		content = parts[1]
	}

	return LogLine{
		Timestamp:     ts,
		Stream:        stream,
		Content:       content,
		ContainerName: containerName,
	}, nil
}
