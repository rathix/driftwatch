package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan manifests and compare against live cluster state",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("scan not yet implemented")
		return nil
	},
}

func init() {
	scanCmd.Flags().String("config", "./driftwatch.yaml", "Config file path")
	scanCmd.Flags().String("kubeconfig", "", "Kubeconfig path (defaults to ~/.kube/config)")
	scanCmd.Flags().String("context", "", "Kubernetes context (defaults to current)")
	scanCmd.Flags().StringSlice("namespace", nil, "Limit to namespace(s)")
	scanCmd.Flags().String("source-type", "auto", "Force source type: manifest, helm, kustomize")
	scanCmd.Flags().String("output", "terminal", "Output format: terminal, json")
	scanCmd.Flags().String("fail-on", "critical", "Severity threshold: critical, warning, info")
	scanCmd.Flags().String("flux", "auto", "Flux enrichment: auto, enabled, disabled")
	rootCmd.AddCommand(scanCmd)
}
