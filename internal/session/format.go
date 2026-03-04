package session

import (
	"encoding/json"
	"time"

	"github.com/Mscotello/docker-agent-tail/internal/docker"
)

// FormatJSONL formats a LogLine as JSON Lines with auto-detection.
// If Content is a valid JSON object, metadata (ts, container, stream) is merged in.
// Otherwise, Content is wrapped in an envelope with a "message" field.
func FormatJSONL(line docker.LogLine) []byte {
	ts := line.Timestamp.UTC().Format(time.RFC3339Nano)

	// Try to detect JSON object content
	content := line.Content
	if len(content) > 0 && content[0] == '{' && json.Valid([]byte(content)) {
		var obj map[string]any
		if err := json.Unmarshal([]byte(content), &obj); err == nil {
			// Merge metadata — overwrite any collisions
			obj["ts"] = ts
			obj["container"] = line.ContainerName
			obj["stream"] = line.Stream
			if data, err := json.Marshal(obj); err == nil {
				return append(data, '\n')
			}
		}
	}

	// Wrap plain text (or non-object JSON) in an envelope
	envelope := map[string]any{
		"ts":        ts,
		"container": line.ContainerName,
		"stream":    line.Stream,
		"message":   content,
	}
	data, _ := json.Marshal(envelope)
	return append(data, '\n')
}
