package handlers

import (
	"io"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
)

// Logs renders the logs page.
func (s *Server) Logs(w http.ResponseWriter, r *http.Request) {
	_, span := s.newSpan(r.Context(), "ui.logs")
	defer span.End()
	span.SetAttributes(attribute.String("ui.page", "logs"))
	defer s.logRequest(r, "logs")

	data := LogsData{
		Active: "logs",
		Title:  "Logs | ClusterProbe",
	}
	if err := s.RenderTemplate(w, "logs", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// LogsStream proxies the API log stream.
func (s *Server) LogsStream(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.logs.stream")
	defer span.End()
	span.SetAttributes(attribute.String("ui.page", "logs"))
	defer s.logRequest(r, "logs_stream")

	stream, err := s.api.LogsStream(ctx)
	if err != nil {
		http.Error(w, "failed to connect log stream", http.StatusBadGateway)
		return
	}
	defer stream.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	buf := make([]byte, 4096)
	for {
		n, readErr := stream.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return
			}
			flusher.Flush()
		}
		if readErr != nil {
			if readErr != io.EOF {
				return
			}
			return
		}
	}
}
