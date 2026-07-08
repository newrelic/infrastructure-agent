package main

import (
	"fmt"
	"os"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "newrelic-infra-config",
	Short: "Offline configuration utility for the New Relic Infrastructure Agent",
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Perform a delta update on a specific key in the newrelic-infra.yml file",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		// Look for the file in the current directory for testing,
		// but default to /etc/newrelic-infra.yml for production.
		configPath := "newrelic-infra.yml"
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			configPath = "/etc/newrelic-infra.yml"
		}

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("configuration file not found at %s", configPath)
		}

		err := config.UpdateConfigKey(configPath, key, value)
		if err != nil {
			return fmt.Errorf("failed to update configuration: %w", err)
		}

		fmt.Printf("Successfully updated '%s' in %s\n", key, configPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configSetCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
