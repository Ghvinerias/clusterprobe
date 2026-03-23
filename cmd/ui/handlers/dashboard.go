package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// Dashboard renders the dashboard page.
func (s *Server) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.dashboard")
	defer span.End()
	span.SetAttributes(attribute.String("ui.page", "dashboard"))
	defer s.logRequest(r, "dashboard")

	stats, _, _ := s.buildStats(ctx, 0, time.Time{})
	data := DashboardData{
		Active: "dashboard",
		Title:  "Dashboard | ClusterProbe",
		Stats:  stats,
	}

	if err := s.RenderTemplate(w, "dashboard", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// DashboardStream streams live stats via SSE.
func (s *Server) DashboardStream(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.dashboard.stream")
	defer span.End()
	span.SetAttributes(attribute.String("ui.page", "dashboard"))
	defer s.logRequest(r, "dashboard_stream")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	prevOps := int64(0)
	prevTime := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(reportingBackoff):
			stats, ops, now := s.buildStats(ctx, prevOps, prevTime)
			prevOps = ops
			prevTime = now

			payload, err := s.renderStats(stats)
			if err != nil {
				continue
			}

			if err := writeSSE(w, "stats", payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) renderStats(stats []Stat) (string, error) {
	tmpl, ok := s.templates["dashboard"]
	if !ok {
		return "", fmt.Errorf("template not found: dashboard")
	}
	buf := &bytes.Buffer{}
	if err := tmpl.ExecuteTemplate(buf, "stat-block", stats); err != nil {
		return "", fmt.Errorf("render stats: %w", err)
	}
	return buf.String(), nil
}

func writeSSE(w http.ResponseWriter, event string, data string) error {
	safe := strings.ReplaceAll(data, "\n", "\ndata: ")
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, safe)
	if err != nil {
		return fmt.Errorf("write sse: %w", err)
	}
	return nil
}
