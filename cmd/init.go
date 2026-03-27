package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
)

var initAPIURL string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Configure opc with your API key and endpoint",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVar(&initAPIURL, "api-url", "", "override the OpsCompanion API URL")
	_ = initCmd.Flags().MarkHidden("api-url")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Check for existing config
	existing, _ := config.Load()
	if existing != nil {
		fmt.Println("Existing configuration found.")
		fmt.Print("Overwrite? [y/N]: ")
		answer, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(answer)) != "y" {
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
