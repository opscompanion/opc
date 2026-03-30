package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
)

var validTimeRanges = map[string]struct{}{
	"1h":  {},
	"6h":  {},
	"12h": {},
	"24h": {},
	"3d":  {},
	"7d":  {},
	"14d": {},
}

var validSearchModes = map[string]struct{}{
	"keyword": {},
	"regex":   {},
}

var validSeverities = map[string]struct{}{
	"TRACE": {},
	"DEBUG": {},
	"INFO":  {},
	"WARN":  {},
	"ERROR": {},
	"FATAL": {},
}

const logsTailPollInterval = 2 * time.Second

type observabilityFlags struct {
	timeRange     string
	mode          string
	caseSensitive bool
	services      []string
	limit         int
	severities    []string
	jsonOutput    bool
	ndjsonOutput  bool
}

func (f observabilityFlags) request(query string, cursor string, includeTimeRange bool) models.ObservabilitySearchRequest {
	req := models.ObservabilitySearchRequest{
		Mode:          f.mode,
		Query:         strings.TrimSpace(query),
		CaseSensitive: f.caseSensitive,
		Services:      cloneStrings(f.services),
		Limit:         f.limit,
		Cursor:        strings.TrimSpace(cursor),
		Severities:    normalizeSeverities(f.severities),
	}
	if includeTimeRange {
		req.TimeRange = f.timeRange
	}
	return req
}

func bindObservabilitySearchFlags(cmd *cobra.Command, flags *observabilityFlags, defaultLimit int) {
	fs := cmd.Flags()
	fs.StringVar(&flags.timeRange, "since", "24h", "time range: 1h, 6h, 12h, 24h, 3d, 7d, 14d")
	bindObservabilityCommonFlags(cmd, flags, defaultLimit)
}

func bindObservabilityTailFlags(cmd *cobra.Command, flags *observabilityFlags, defaultLimit int) {
	bindObservabilityCommonFlags(cmd, flags, defaultLimit)
}

func bindObservabilityCommonFlags(cmd *cobra.Command, flags *observabilityFlags, defaultLimit int) {
	fs := cmd.Flags()
	fs.StringVar(&flags.mode, "mode", "keyword", "search mode: keyword or regex")
	fs.BoolVar(&flags.caseSensitive, "case-sensitive", false, "enable case-sensitive search")
	fs.StringSliceVar(&flags.services, "service", nil, "service name filter (repeatable)")
	fs.IntVar(&flags.limit, "limit", defaultLimit, "maximum number of results")
	fs.StringSliceVar(&flags.severities, "severity", nil, "severity filter (repeatable): TRACE, DEBUG, INFO, WARN, ERROR, FATAL")
	fs.BoolVar(&flags.jsonOutput, "json", false, "output the full API response as JSON")
	fs.BoolVar(&flags.ndjsonOutput, "ndjson", false, "output one result object per line")
}

func validateObservabilitySearchFlags(flags observabilityFlags) error {
	if err := validateObservabilityCommonFlags(flags); err != nil {
		return err
	}
	if _, ok := validTimeRanges[flags.timeRange]; !ok {
		return fmt.Errorf("invalid --since %q (expected 1h, 6h, 12h, 24h, 3d, 7d, or 14d)", flags.timeRange)
	}
	return nil
}

func validateObservabilityTailFlags(flags observabilityFlags) error {
	return validateObservabilityCommonFlags(flags)
}

func validateObservabilityCommonFlags(flags observabilityFlags) error {
	if strings.TrimSpace(flags.timeRange) != "" {
		if _, ok := validTimeRanges[flags.timeRange]; !ok {
			return fmt.Errorf("invalid --since %q (expected 1h, 6h, 12h, 24h, 3d, 7d, or 14d)", flags.timeRange)
		}
	}
	if _, ok := validSearchModes[flags.mode]; !ok {
		return fmt.Errorf("invalid --mode %q (expected keyword or regex)", flags.mode)
	}
	if flags.limit <= 0 || flags.limit > 500 {
		return fmt.Errorf("invalid --limit %d (expected 1-500)", flags.limit)
	}
	if flags.jsonOutput && flags.ndjsonOutput {
		return errors.New("cannot use --json and --ndjson together")
	}
	for _, severity := range flags.severities {
		upper := strings.ToUpper(strings.TrimSpace(severity))
		if _, ok := validSeverities[upper]; !ok {
			return fmt.Errorf("invalid --severity %q (expected TRACE, DEBUG, INFO, WARN, ERROR, or FATAL)", severity)
		}
	}
	return nil
}

func normalizeSeverities(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToUpper(strings.TrimSpace(value))
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func joinQuery(args []string) string {
	return strings.TrimSpace(strings.Join(args, " "))
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func writeNDJSON[T any](w io.Writer, entries []T) error {
	enc := json.NewEncoder(w)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			return err
		}
	}
	return nil
}

func formatTimestamp(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "-"
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return "-"
		}
		return text
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	return strings.TrimSpace(string(raw))
}

func quoteField(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	value = strings.ReplaceAll(value, `"`, `'`)
	if value == "" {
		return `""`
	}
	return strconv.Quote(value)
}

func shortID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func formatDurationMicros(value float64) string {
	if value <= 0 {
		return "0ms"
	}
	d := time.Duration(value) * time.Microsecond
	if d >= time.Second {
		return d.Round(time.Millisecond).String()
	}
	if d >= time.Millisecond {
		return d.Round(time.Millisecond).String()
	}
	return d.String()
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func renderFilterSummary(query string, flags observabilityFlags) string {
	parts := []string{"mode=" + flags.mode}
	if strings.TrimSpace(flags.timeRange) != "" {
		parts = append(parts, "since="+flags.timeRange)
	}
	if query != "" {
		parts = append(parts, "query="+quoteField(query))
	}
	if len(flags.services) > 0 {
		parts = append(parts, "services="+strings.Join(flags.services, ","))
	}
	if len(flags.severities) > 0 {
		parts = append(parts, "severities="+strings.Join(normalizeSeverities(flags.severities), ","))
	}
	parts = append(parts, fmt.Sprintf("limit=%d", flags.limit))
	if flags.caseSensitive {
		parts = append(parts, "case-sensitive=true")
	}
	return strings.Join(parts, " ")
}

func renderTUIInfo(flags observabilityFlags) string {
	parts := make([]string, 0, 3)
	if len(flags.services) > 0 {
		parts = append(parts, "service="+strings.Join(flags.services, ","))
	}
	if flags.mode != "keyword" {
		parts = append(parts, "mode="+flags.mode)
	}
	if flags.caseSensitive {
		parts = append(parts, "case-sensitive")
	}
	return strings.Join(parts, "  |  ")
}

func renderTUIDetails(query string, flags observabilityFlags) string {
	parts := []string{fmt.Sprintf("limit=%d", flags.limit)}
	if query != "" {
		parts = append(parts, "query="+quoteField(query))
	}
	if len(flags.severities) > 0 {
		parts = append(parts, "severity="+strings.Join(normalizeSeverities(flags.severities), ","))
	}
	return strings.Join(parts, "  |  ")
}
