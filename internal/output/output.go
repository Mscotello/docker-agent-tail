// Package output provides terminal output formatting and coloring.
package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/Mscotello/docker-agent-tail/internal/docker"
)

// OutputWriter handles terminal output with colors and filtering.
type OutputWriter struct {
	w               io.Writer
	noColor         bool
	muteContainers  map[string]bool
	containerColors map[string]func(string, ...interface{}) string
	mu              sync.Mutex
}

// NewOutputWriter creates a new output writer.
// noColor disables colors. noColorEnv respects NO_COLOR environment variable per https://no-color.org
func NewOutputWriter(w io.Writer, noColor bool, muteContainers []string) *OutputWriter {
	// Check NO_COLOR env var
	if os.Getenv("NO_COLOR") != "" {
		noColor = true
	}

	muteMap := make(map[string]bool)
	for _, c := range muteContainers {
		muteMap[c] = true
	}

	return &OutputWriter{
		w:               w,
		noColor:         noColor,
		muteContainers:  muteMap,
		containerColors: make(map[string]func(string, ...interface{}) string),
	}
}

// colorFuncs are a predefined set of color functions for containers.
var colorFuncs = []func(string, ...interface{}) string{
	color.CyanString,
	color.MagentaString,
	color.YellowString,
	color.GreenString,
	color.BlueString,
	color.RedString,
}

// getContainerColor assigns a consistent color to a container name.
// Must be called with ow.mu held.
func (ow *OutputWriter) getContainerColor(containerName string) func(string, ...interface{}) string {
	if fn, exists := ow.containerColors[containerName]; exists {
		return fn
	}

	// Assign next color based on number of containers seen
	idx := len(ow.containerColors) % len(colorFuncs)
	fn := colorFuncs[idx]
	ow.containerColors[containerName] = fn
	return fn
}

// WriteLogLine writes a formatted log line to terminal.
// Respects --mute for containers that shouldn't be displayed.
func (ow *OutputWriter) WriteLogLine(line docker.LogLine) {
	if ow.muteContainers[line.ContainerName] {
		return
	}

	ow.mu.Lock()
	defer ow.mu.Unlock()

	ts := line.Timestamp.Format("15:04:05")
	containerName := line.ContainerName
	stream := line.Stream
	if !ow.noColor {
		colorFn := ow.getContainerColor(line.ContainerName)
		containerName = colorFn(line.ContainerName)
		if stream == "stderr" {
			stream = color.RedString(stream)
		}
	}

	fmt.Fprintf(ow.w, "[%s] %-12s [%s] %s\n", ts, containerName, stream, line.Content)
}

// WriteJSON writes a log line in JSON format.
// Respects --mute for containers that shouldn't be displayed.
func (ow *OutputWriter) WriteJSON(line docker.LogLine) error {
	if ow.muteContainers[line.ContainerName] {
		return nil
	}

	ow.mu.Lock()
	defer ow.mu.Unlock()

	// Simple JSON format
	jsonLine := fmt.Sprintf(`{"timestamp":"%s","container":"%s","stream":"%s","message":%q}`,
		line.Timestamp.Format("2006-01-02T15:04:05.999Z07:00"),
		line.ContainerName,
		line.Stream,
		line.Content,
	)
	_, err := fmt.Fprintf(ow.w, "%s\n", jsonLine)
	return err
}

// ParseLogLine is a helper for testing; parses a simple log line.
// Used by tests to verify formatting.
type ParsedLogLine struct {
	Timestamp     string
	ContainerName string
	Stream        string
	Content       string
}

// ParseTerminalLine parses a terminal output line back to components (for testing).
func ParseTerminalLine(line string) (ParsedLogLine, error) {
	// Format: [HH:MM:SS] container_name   [stream] content
	parts := strings.SplitN(line, "]", 4)
	if len(parts) < 4 {
		return ParsedLogLine{}, fmt.Errorf("invalid log line format")
	}

	ts := strings.TrimPrefix(parts[0], "[")
	containerName := strings.TrimSpace(parts[1])
	streamContent := strings.SplitN(parts[2], " [", 2)
	if len(streamContent) < 2 {
		return ParsedLogLine{}, fmt.Errorf("invalid stream format")
	}

	stream := strings.TrimPrefix(strings.TrimSuffix(streamContent[1], "]"), "")
	content := strings.TrimSpace(parts[3])

	return ParsedLogLine{
		Timestamp:     ts,
		ContainerName: containerName,
		Stream:        stream,
		Content:       content,
	}, nil
}
