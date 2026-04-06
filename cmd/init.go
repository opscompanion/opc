package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var initAPIURL string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Legacy config-only setup (soon to be deprecated)",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVar(&initAPIURL, "api-url", "", "override the OpsCompanion API URL")
	_ = initCmd.Flags().MarkHidden("api-url")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check for existing config
	existing, _ := config.Load()
	if existing != nil {
		fmt.Println("Existing configuration found.")
		confirmed, err := promptConfirmAction("confirm", "Overwrite the saved config?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Collect inputs
	fmt.Print("API Key: ")
	apiKeyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("reading API key: %w", err)
	}
	apiKey := strings.TrimSpace(string(apiKeyBytes))
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	cfg := &models.Config{
		APIKey: apiKey,
	}
	if strings.TrimSpace(initAPIURL) != "" {
		cfg.APIURL = strings.TrimRight(strings.TrimSpace(initAPIURL), "/")
	} else if envURL := strings.TrimSpace(os.Getenv("OPSCOMPANION_API_URL")); envURL != "" {
		cfg.APIURL = strings.TrimRight(envURL, "/")
	} else if existing != nil && strings.TrimSpace(existing.APIURL) != "" {
		cfg.APIURL = strings.TrimRight(strings.TrimSpace(existing.APIURL), "/")
	} else {
		cfg.APIURL = config.DefaultAPIURL
	}

	client := api.New(cfg)
	whoami, err := client.Verify()
	if err != nil {
		return fmt.Errorf("verifying API key with %s: %w", cfg.APIURL, err)
	}

	// Save
	if err := config.Save(cfg); err != nil {
		return err
	}

	masked := apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
	if len(apiKey) < 10 {
		masked = "****"
	}

	fmt.Println()
	fmt.Println("Configuration saved.")
	fmt.Printf("  API URL: %s\n", cfg.APIURL)
	fmt.Printf("  API Key: %s\n", masked)
	fmt.Printf("  Owner:   %s\n", whoami.APIKey.OwnerType)
	fmt.Printf("  Org:     %s\n", whoami.Organization.PublicID)
	if whoami.User != nil {
		fmt.Printf("  User:    %s\n", whoami.User.PublicID)
	}
	if len(whoami.APIKey.Scopes) > 0 {
		fmt.Printf("  Scopes:  %s\n", strings.Join(whoami.APIKey.Scopes, ", "))
	}

	p, _ := config.Path()
	fmt.Printf("  File:    %s\n", p)
	fmt.Println()
	fmt.Println("Run `opc context` to verify your setup.")
	return nil
}
