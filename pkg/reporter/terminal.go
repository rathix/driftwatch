package reporter

import (
	"fmt"
	"io"

	"github.com/kennyandries/driftwatch/pkg/types"
)

type TerminalReporter struct {
	w     io.Writer
	color bool
}

func NewTerminalReporter(w io.Writer, color bool) *TerminalReporter {
	return &TerminalReporter{w: w, color: color}
}

func (tr *TerminalReporter) Report(results []types.DriftResult) error {
	inSync, drifted, missing := 0, 0, 0

	for _, result := range results {
		switch result.Status {
		case types.StatusInSync:
			inSync++
			fmt.Fprintf(tr.w, "✓ %s: in sync\n", result.ID)

		case types.StatusDrifted:
			drifted++
			fmt.Fprintf(tr.w, "✗ %s: drifted [%s]\n", result.ID, result.Severity)
			for _, diff := range result.Diffs {
				fmt.Fprintf(tr.w, "  - %s [%s]\n", diff.Path, diff.Severity)
				fmt.Fprintf(tr.w, "    Expected: %s\n", sanitizeOutput(diff.Expected))
				fmt.Fprintf(tr.w, "    Actual:   %s\n", sanitizeOutput(diff.Actual))
			}
			if result.FluxStatus != nil {
				fmt.Fprintf(tr.w, "  Flux: Ready=%v, Suspended=%v\n", result.FluxStatus.Ready, result.FluxStatus.Suspended)
			}

		case types.StatusMissing:
			missing++
			fmt.Fprintf(tr.w, "⊘ %s: missing [%s]\n", result.ID, result.Severity)
			if result.FluxStatus != nil {
				fmt.Fprintf(tr.w, "  Flux: Ready=%v, Suspended=%v\n", result.FluxStatus.Ready, result.FluxStatus.Suspended)
			}
		}
	}

	total := inSync + drifted + missing
	fmt.Fprintf(tr.w, "\nSummary: %d total, %d in sync, %d drifted, %d missing\n",
		total, inSync, drifted, missing)

	return nil
}

func sanitizeOutput(s string) string {
	return string([]rune(s))
}
