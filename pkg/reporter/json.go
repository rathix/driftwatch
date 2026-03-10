package reporter

import (
	"encoding/json"
	"io"
	"time"

	"github.com/kennyandries/driftwatch/pkg/types"
)

type JSONReporter struct {
	w io.Writer
}

type jsonMetadata struct {
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

type jsonSummary struct {
	Total   int `json:"total"`
	InSync  int `json:"in_sync"`
	Drifted int `json:"drifted"`
	Missing int `json:"missing"`
}

type jsonOutput struct {
	Metadata jsonMetadata      `json:"metadata"`
	Results  []types.DriftResult `json:"results"`
	Summary  jsonSummary       `json:"summary"`
}

func NewJSONReporter(w io.Writer) *JSONReporter {
	return &JSONReporter{w: w}
}

func (jr *JSONReporter) Report(results []types.DriftResult) error {
	inSync, drifted, missing := 0, 0, 0

	for _, result := range results {
		switch result.Status {
		case types.StatusInSync:
			inSync++
		case types.StatusDrifted:
			drifted++
		case types.StatusMissing:
			missing++
		}
	}

	output := jsonOutput{
		Metadata: jsonMetadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Version:   "1.0",
		},
		Results: results,
		Summary: jsonSummary{
			Total:   len(results),
			InSync:  inSync,
			Drifted: drifted,
			Missing: missing,
		},
	}

	encoder := json.NewEncoder(jr.w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(output)
}
