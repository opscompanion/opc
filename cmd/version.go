package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/opscompanion/opc/internal/update"
	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var versionJSON bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of opc",
	RunE: func(cmd *cobra.Command, args []string) error {
		hint := update.Check(Version, true)
		latest := ""
		updateAvailable := false
		if hint != "" {
			updateAvailable = true
			// Extract latest version from hint message
			latest = extractLatest(hint)
		}

		if versionJSON {
			out := struct {
				Version         string `json:"version"`
				Commit          string `json:"commit"`
				Date            string `json:"date"`
				Latest          string `json:"latest,omitempty"`
				UpdateAvailable bool   `json:"update_available"`
			}{
				Version:         Version,
				Commit:          Commit,
				Date:            Date,
				Latest:          latest,
				UpdateAvailable: updateAvailable,
			}
			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("opc %s (commit: %s, built: %s)\n", Version, Commit, Date)
		if hint != "" {
			fmt.Println("Update available: " + latest + " — run `brew upgrade opc`")
		}
		return nil
	},
}

// extractLatest pulls the target version from a hint like "opc: update available v0.2.1 → v0.3.1 — ..."
func extractLatest(hint string) string {
	// Find "→ " and extract the version after it
	const arrow = "→ "
	idx := 0
	for i := 0; i < len(hint)-len(arrow); i++ {
		if hint[i:i+len(arrow)] == arrow {
			idx = i + len(arrow)
			break
		}
	}
	if idx == 0 {
		return ""
	}
	rest := hint[idx:]
	// Version ends at space or end of string
	for i, c := range rest {
		if c == ' ' {
			return rest[:i]
		}
	}
	return rest
}

func init() {
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "output version info as JSON")
	rootCmd.AddCommand(versionCmd)
}
