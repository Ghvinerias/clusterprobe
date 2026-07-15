package handlers

import (
	"fmt"
	"html"
	"net/http"
	"time"
)

// LogsHandler handles live log stream endpoints.
type LogsHandler struct {
	now func() time.Time
}

// NewLogsHandler builds a LogsHandler.
func NewLogsHandler() *LogsHandler {
	return &LogsHandler{now: time.Now}
}

// Stream handles GET /api/v1/logs/stream.
func (h *LogsHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		errorResponse(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	writeLogEvent(w, h.now(), "clusterprobe-api log stream connected")
	flusher.Flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ts := <-ticker.C:
			writeLogEvent(w, ts, "clusterprobe-api log stream heartbeat")
			flusher.Flush()
		}
	}
}

func writeLogEvent(w http.ResponseWriter, ts time.Time, message string) {
	line := fmt.Sprintf(
		`<div class="log-line"><span>%s</span> %s</div>`,
		html.EscapeString(ts.UTC().Format(time.RFC3339)),
		html.EscapeString(message),
	)
	_, _ = fmt.Fprintf(w, "event: logs\ndata: %s\n\n", line)
}
