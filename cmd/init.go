package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Configure opc with your API key and endpoint",
	RunE:  runInit,
}

func init() {
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
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	fmt.Print("API URL [https://api.opscompanion.dev/v1]: ")
	apiURL, _ := reader.ReadString('\n')
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		apiURL = "https://api.opscompanion.dev/v1"
	}

	cfg := &models.Config{
		APIURL: apiURL,
		APIKey: apiKey,
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
	fmt.Printf("  API URL: %s\n", apiURL)
	fmt.Printf("  API Key: %s\n", masked)

	p, _ := config.Path()
	fmt.Printf("  File:    %s\n", p)
	fmt.Println()
	fmt.Println("Run `opc context` to verify your setup.")
	return nil
}
