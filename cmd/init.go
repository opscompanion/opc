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
var initProfile string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Configure opc with your API key and endpoint",
	Long: `Configure opc with your API key and endpoint.

Without --profile, configures the [default] profile.
With --profile <name>, configures a named profile and associates the
current directory with it.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initAPIURL, "api-url", "", "override the OpsCompanion API URL")
	initCmd.Flags().StringVar(&initProfile, "profile", "", "profile name to configure (omit for default)")
	_ = initCmd.Flags().MarkHidden("api-url")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	profileLabel := "default"
	if initProfile != "" {
		profileLabel = initProfile
	}

	// Check for existing config.
	tc, _ := config.LoadFull()
	if tc != nil {
		var exists bool
		if initProfile == "" {
			exists = tc.Default.APIKey != ""
		} else {
			_, exists = tc.Profiles[initProfile]
		}
		if exists {
			fmt.Printf("Existing configuration found for profile %q.\n", profileLabel)
			fmt.Print("Overwrite? [y/N]: ")
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}
	}

	// Collect inputs.
	fmt.Printf("Configuring profile: %s\n", profileLabel)
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

	// Resolve API URL: flag > env > existing config > default.
	apiURL := resolveInitAPIURL(tc, initProfile)

	cfg := &models.Config{
		APIKey: apiKey,
		APIURL: apiURL,
	}

	client := api.New(cfg)
	whoami, err := client.Verify()
	if err != nil {
		return fmt.Errorf("verifying API key with %s: %w", cfg.APIURL, err)
	}

	// Save to the appropriate profile.
	if err := config.SaveProfile(cfg, initProfile); err != nil {
		return err
	}

	// If a named profile was specified, auto-add cwd to its paths.
	if initProfile != "" {
		cwd, err := os.Getwd()
		if err == nil {
			if err := config.AddProfilePath(initProfile, cwd); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not add path to profile: %v\n", err)
			}
		}
	}

	masked := apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
	if len(apiKey) < 10 {
		masked = "****"
	}

	fmt.Println()
	fmt.Println("Configuration saved.")
	fmt.Printf("  Profile: %s\n", profileLabel)
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
	if initProfile != "" {
		cwd, _ := os.Getwd()
		fmt.Printf("  Path:    %s (auto-linked)\n", cwd)
	}
	fmt.Println()
	fmt.Println("Run `opc context` to verify your setup.")
	return nil
}

// resolveInitAPIURL determines the API URL for the init command using
// precedence: --api-url flag > env > existing profile config > default.
func resolveInitAPIURL(tc *config.TOMLConfig, profileName string) string {
	if u := strings.TrimSpace(initAPIURL); u != "" {
		return strings.TrimRight(u, "/")
	}
	if envURL := strings.TrimSpace(os.Getenv("OPSCOMPANION_API_URL")); envURL != "" {
		return strings.TrimRight(envURL, "/")
	}
	if tc != nil {
		if profileName == "" {
			if u := strings.TrimSpace(tc.Default.APIURL); u != "" {
				return strings.TrimRight(u, "/")
			}
		} else if p, ok := tc.Profiles[profileName]; ok {
			if u := strings.TrimSpace(p.APIURL); u != "" {
				return strings.TrimRight(u, "/")
			}
			// Inherit from default.
			if u := strings.TrimSpace(tc.Default.APIURL); u != "" {
				return strings.TrimRight(u, "/")
			}
		}
	}
	return config.DefaultAPIURL
}
