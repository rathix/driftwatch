package cmd

import (
	"fmt"

	"github.com/kennyandries/driftwatch/pkg/config"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config file without scanning",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		_, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("config invalid: %w", err)
		}
		fmt.Println("Config valid")
		return nil
	},
}

func init() {
	validateCmd.Flags().String("config", "./driftwatch.yaml", "Config file path")
	rootCmd.AddCommand(validateCmd)
}
