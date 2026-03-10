package reporter

import "github.com/kennyandries/driftwatch/pkg/types"

type Reporter interface {
	Report(results []types.DriftResult) error
}

func ExitCode(results []types.DriftResult, threshold types.Severity) int {
	for _, result := range results {
		if result.Status == types.StatusDrifted || result.Status == types.StatusMissing {
			if result.ExceedsThreshold(threshold) {
				return 1
			}
		}
	}
	return 0
}
