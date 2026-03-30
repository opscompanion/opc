package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
)

var tracesFlags observabilityFlags

var tracesCmd = &cobra.Command{
	Use:   "traces [query]",
	Short: "Search traces",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runTraces,
}

func init() {
	bindObservabilitySearchFlags(tracesCmd, &tracesFlags, 50)
	rootCmd.AddCommand(tracesCmd)
}

func runTraces(cmd *cobra.Command, args []string) error {
	if err := validateObservabilitySearchFlags(tracesFlags); err != nil {
		return err
	}

	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	client := api.New(cfg)
	resp, err := client.SearchTraces(tracesFlags.request(joinQuery(args), "", true))
	if err != nil {
		return fmt.Errorf("searching traces: %w", err)
	}
	return writeTracesOutput(cmd.OutOrStdout(), resp, tracesFlags)
}

func writeTracesOutput(w io.Writer, resp *models.TracesResult, flags observabilityFlags) error {
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
			if _, err := fmt.Fprintln(w, formatTraceEntry(entry)); err != nil {
				return err
			}
		}
		return nil
	}
}

func formatTraceEntry(entry models.TraceEntry) string {
	status := fmt.Sprintf("%d", entry.StatusCode)
	if strings.TrimSpace(entry.StatusMessage) != "" {
		status += ":" + strings.Join(strings.Fields(entry.StatusMessage), " ")
	}
	return fmt.Sprintf(
		"%s service=%s span=%s duration=%s status=%s trace=%s span_id=%s",
		formatTimestamp(entry.Timestamp),
		valueOrDash(entry.ServiceName),
		quoteField(entry.SpanName),
		formatDurationMicros(entry.Duration),
		status,
		shortID(entry.TraceID),
		shortID(entry.SpanID),
	)
}
