package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Mscotello/docker-agent-tail/internal/docker"
)

// levelAliases lists field names to check for severity level, in priority order.
var levelAliases = []string{"level", "lvl", "severity", "s", "sev", "log_level", "levelname"}

// levelNormMap maps raw level values to lnav-standard names.
var levelNormMap = map[string]string{
	"i": "info", "info": "info", "notice": "info",
	"w": "warn", "warn": "warning", "warning": "warning",
	"e": "err", "err": "error", "error": "error",
	"f": "fatal", "fatal": "fatal", "critical": "fatal", "crit": "fatal", "emerg": "fatal",
	"d": "debug", "debug": "debug",
	"t": "trace", "trace": "trace",
}

// normalizeLevel extracts the level from known aliases, removes the alias key,
// and sets a canonical "level" field with a normalized value.
func normalizeLevel(obj map[string]any) {
	for _, alias := range levelAliases {
		raw, ok := obj[alias]
		if !ok {
			continue
		}
		str, ok := raw.(string)
		if !ok {
			break
		}
		// Remove alias key if it's not "level"
		if alias != "level" {
			delete(obj, alias)
		}
		// Normalize value
		lower := strings.ToLower(str)
		if normalized, found := levelNormMap[lower]; found {
			obj["level"] = normalized
		} else {
			obj["level"] = lower
		}
		return
	}
}

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
			// Normalize level before merging metadata
			normalizeLevel(obj)

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
	data, err := json.Marshal(envelope)
	if err != nil {
		// Fall back to a minimal valid JSON line on marshal error
		fallback := fmt.Sprintf(`{"ts":%q,"container":%q,"message":"marshal error"}`, ts, line.ContainerName)
		return append([]byte(fallback), '\n')
	}
	return append(data, '\n')
}
