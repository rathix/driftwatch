package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config file without scanning",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("validate not yet implemented")
		return nil
	},
}

func init() {
	validateCmd.Flags().String("config", "./driftwatch.yaml", "Config file path")
	rootCmd.AddCommand(validateCmd)
}
