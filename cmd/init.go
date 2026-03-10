package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const defaultConfig = `# driftwatch configuration
sources: []
ignore:
  fields:
    - "metadata.managedFields"
    - "metadata.resourceVersion"
    - "metadata.uid"
    - "metadata.generation"
    - "metadata.creationTimestamp"
    - "status"
  resources:
    - kind: Secret
severity:
  critical:
    - "spec.containers.*.image"
    - "spec.template.spec.containers.*.image"
    - "rules"
  warning:
    - "spec.replicas"
    - "spec.template.spec.resources"
cluster:
  kubeconfig: ""
  context: ""
flux:
  enabled: true
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a starter driftwatch.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		const filename = "driftwatch.yaml"
		if _, err := os.Stat(filename); err == nil {
			return fmt.Errorf("%s already exists", filename)
		}
		if err := os.WriteFile(filename, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
		fmt.Printf("Created %s\n", filename)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
