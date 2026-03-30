package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/opscompanion/opc/internal/models"
)

func TestValidateObservabilitySearchFlags(t *testing.T) {
	err := validateObservabilitySearchFlags(observabilityFlags{
		timeRange: "2h",
		mode:      "keyword",
		limit:     10,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid --since") {
		t.Fatalf("validateObservabilitySearchFlags() error = %v", err)
	}

	err = validateObservabilitySearchFlags(observabilityFlags{
		timeRange: "1h",
		mode:      "regex",
		limit:     10,
		severities: []string{
			"warn",
			"panic",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid --severity") {
		t.Fatalf("validateObservabilitySearchFlags() error = %v", err)
	}
}

func TestValidateObservabilityTailFlags(t *testing.T) {
	err := validateObservabilityTailFlags(observabilityFlags{
		mode:  "keyword",
		limit: 10,
	})
	if err != nil {
		t.Fatalf("validateObservabilityTailFlags() error = %v", err)
	}

	err = validateObservabilityTailFlags(observabilityFlags{
		timeRange: "2h",
		mode:      "keyword",
		limit:     10,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid --since") {
		t.Fatalf("validateObservabilityTailFlags() error = %v", err)
	}
}

func TestWriteLogsOutputNDJSON(t *testing.T) {
	var buf bytes.Buffer
	resp := &models.LogsResult{
		Data: []models.LogEntry{
			{
				Timestamp:    []byte(`"2026-03-30T12:00:00Z"`),
				TraceID:      "trace1234567890",
				SpanID:       "span1234567890",
				SeverityText: "ERROR",
				Body:         "timeout waiting for upstream",
				ServiceName:  "api",
			},
		},
	}

	if err := writeLogsOutput(&buf, resp, observabilityFlags{ndjsonOutput: true}); err != nil {
		t.Fatalf("writeLogsOutput: %v", err)
	}
	if !strings.Contains(buf.String(), `"service_name":"api"`) {
		t.Fatalf("writeLogsOutput() = %q", buf.String())
	}
}

func TestFormatters(t *testing.T) {
	logLine := formatLogEntry(models.LogEntry{
		Timestamp:    []byte(`"2026-03-30T12:00:00Z"`),
		TraceID:      "trace1234567890",
		SpanID:       "span1234567890",
		SeverityText: "WARN",
		Body:         "multi\nline body",
		ServiceName:  "api",
	})
	if !strings.Contains(logLine, `body="multi line body"`) {
		t.Fatalf("formatLogEntry() = %q", logLine)
	}

	traceLine := formatTraceEntry(models.TraceEntry{
		Timestamp:     []byte(`1711800000`),
		TraceID:       "trace1234567890",
		SpanID:        "span1234567890",
		SpanName:      "GET /health",
		Duration:      1500,
		StatusCode:    1,
		StatusMessage: "ok",
		ServiceName:   "api",
	})
	if !strings.Contains(traceLine, `duration=2ms`) || !strings.Contains(traceLine, `span="GET /health"`) {
		t.Fatalf("formatTraceEntry() = %q", traceLine)
	}
}

func TestRenderTUIHelpers(t *testing.T) {
	info := renderTUIInfo(observabilityFlags{
		services: []string{"vercel-app"},
		mode:     "regex",
	})
	if !strings.Contains(info, "service=vercel-app") || !strings.Contains(info, "mode=regex") {
		t.Fatalf("renderTUIInfo() = %q", info)
	}

	details := renderTUIDetails("timeout", observabilityFlags{
		limit:      20,
		severities: []string{"error"},
	})
	if !strings.Contains(details, "limit=20") || !strings.Contains(details, `query="timeout"`) || !strings.Contains(details, "severity=ERROR") {
		t.Fatalf("renderTUIDetails() = %q", details)
	}
}

func TestFormatLogEntryForTUI(t *testing.T) {
	line := formatLogEntryForTUI(models.LogEntry{
		Timestamp:    []byte(`"2026-03-30T12:00:00Z"`),
		SeverityText: "ERROR",
		ServiceName:  "api",
		Body:         "boom",
	})
	if !strings.Contains(line, "2026-03-30T12:00:00Z") || !strings.Contains(line, "api") || !strings.Contains(line, `"boom"`) {
		t.Fatalf("formatLogEntryForTUI() = %q", line)
	}
}
