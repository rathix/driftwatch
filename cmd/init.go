package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a starter driftwatch.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("init not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
