package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// FileResult is the outcome of creating or updating one file.
type FileResult struct {
	Path   string `json:"path" yaml:"path"`
	Status string `json:"status" yaml:"status"`
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

func outputFileResults(cmd *cobra.Command, results []FileResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(results, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(results)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		for _, r := range results {
			line := fmt.Sprintf("[%s] %s", r.Status, r.Path)
			if r.Reason != "" {
				line += "  # " + r.Reason
			}
			outputString(cmd, line+"\n")
		}
	}
}
