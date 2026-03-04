// Package docker provides Docker SDK client operations.
package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
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

		// Demux if not TTY, otherwise read raw
		var reader io.Reader
		if resp.Config.Tty {
			reader = logs
		} else {
			// stdcopy demultiplexes stdout/stderr
			outReader, outWriter := io.Pipe()
			errReader, errWriter := io.Pipe()

			go func() {
				if _, err := stdcopy.StdCopy(outWriter, errWriter, logs); err != nil {
					errCh <- fmt.Errorf("demuxing logs: %w", err)
				}
				outWriter.Close()
				errWriter.Close()
			}()

			// Combine both streams for processing
			// For now, process stdout only - stderr will be handled separately
			reader = outReader
			_ = errReader // TODO: handle stderr stream separately if needed
		}

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			logLine, err := parseLogLine(line, opts.ContainerName)
			if err != nil {
				continue
			}
			logCh <- logLine
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("reading logs: %w", err)
		}
	}()

	return logCh, errCh
}

// parseLogLine parses a Docker log line with timestamp.
// Format: "2006-01-02T15:04:05.999999999Z content"
func parseLogLine(line, containerName string) (LogLine, error) {
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
		Stream:        "stdout",
		Content:       content,
		ContainerName: containerName,
	}, nil
}
