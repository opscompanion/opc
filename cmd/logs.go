package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
)

var logsFlags observabilityFlags
var logsTailFlags observabilityFlags
var logsTailTUI bool

var logsCmd = &cobra.Command{
	Use:   "logs [query]",
	Short: "Search logs",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLogs,
}

var logsTailCmd = &cobra.Command{
	Use:   "tail [query]",
	Short: "Poll for new logs",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLogsTail,
}

func init() {
	bindObservabilitySearchFlags(logsCmd, &logsFlags, 50)
	rootCmd.AddCommand(logsCmd)

	bindObservabilityTailFlags(logsTailCmd, &logsTailFlags, 20)
	logsTailCmd.Flags().BoolVar(&logsTailTUI, "tui", false, "render a lightweight terminal UI")
	logsCmd.AddCommand(logsTailCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	if err := validateObservabilitySearchFlags(logsFlags); err != nil {
		return err
	}

	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	client := api.New(cfg)
	resp, err := client.SearchLogs(logsFlags.request(joinQuery(args), "", true))
	if err != nil {
		return fmt.Errorf("searching logs: %w", err)
	}
	return writeLogsOutput(cmd.OutOrStdout(), resp, logsFlags)
}

func runLogsTail(cmd *cobra.Command, args []string) error {
	if err := validateObservabilityTailFlags(logsTailFlags); err != nil {
		return err
	}
	if logsTailTUI && (logsTailFlags.jsonOutput || logsTailFlags.ndjsonOutput) {
		return fmt.Errorf("cannot use --tui with --json or --ndjson")
	}

	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	client := api.New(cfg)
	query := joinQuery(args)
	ctx, cancel := signalContext()
	defer cancel()

	if logsTailTUI && canUseBubbleTailTUI() {
		return runLogsTailTea(ctx, client, query, logsTailFlags)
	}

	cursor := ""
	for {
		resp, err := client.TailLogs(logsTailFlags.request(query, cursor, false))
		if err != nil {
			return fmt.Errorf("tailing logs: %w", err)
		}
		if err := writeLogsOutput(cmd.OutOrStdout(), resp, logsTailFlags); err != nil {
			return err
		}
		if resp.NextCursor != nil && strings.TrimSpace(*resp.NextCursor) != "" {
			cursor = *resp.NextCursor
		}
		if resp.HasMore {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(logsTailPollInterval):
		}
	}
}

func writeLogsOutput(w io.Writer, resp *models.LogsResult, flags observabilityFlags) error {
	switch {
	case flags.jsonOutput:
		return writeJSON(w, resp)
	case flags.ndjsonOutput:
		return writeNDJSON(w, resp.Data)
	default:
		if len(resp.Data) == 0 {
			return nil
		}
		for _, entry := range resp.Data {
			if _, err := fmt.Fprintln(w, formatLogEntry(entry)); err != nil {
				return err
			}
		}
		return nil
	}
}

func formatLogEntry(entry models.LogEntry) string {
	return fmt.Sprintf(
		"%s service=%s severity=%s trace=%s span=%s body=%s",
		formatTimestamp(entry.Timestamp),
		valueOrDash(entry.ServiceName),
		valueOrDash(entry.SeverityText),
		shortID(entry.TraceID),
		shortID(entry.SpanID),
		quoteField(entry.Body),
	)
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
