package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "driftwatch",
	Short: "Detect Kubernetes config drift between Git and live cluster",
	Long:  "Driftwatch compares Git-stored manifests (YAML, Helm, Kustomize) against live cluster state and flags any drift.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
