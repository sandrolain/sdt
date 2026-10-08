package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/semantic"
)

// ctxSnapshotBuildVerb names the build subcommand (a constant so the repeated
// literal is not flagged by goconst).
const ctxSnapshotBuildVerb = "build"

// ctxSnapshotResult is the build command's output payload.
type ctxSnapshotResult struct {
	Path     string `json:"path" yaml:"path"`
	Model    string `json:"model" yaml:"model"`
	Sections int    `json:"sections" yaml:"sections"`
	Reused   int    `json:"reused" yaml:"reused"`
	Embedded int    `json:"embedded" yaml:"embedded"`
}

var contextSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage the derived semantic vector snapshot",
	Long: `Manage the derived semantic vector snapshot (vectors.gob.gz under
.sdt/cache) that backs the offline neighbour surfaces (sdt context related, the
viewer semantic map).`,
}

var contextSnapshotBuildCmd = &cobra.Command{
	Use:   ctxSnapshotBuildVerb,
	Short: "Build or refresh the semantic vector snapshot",
	Long: `Build the semantic vector snapshot over the current context/ search index,
reusing unchanged vectors from the existing snapshot. No query is needed; the
embedding model is loaded from the local cache (default BASE8M, override with
--semantic-model).

Examples:
  sdt context snapshot build
  sdt context snapshot build --semantic-model BASE2M --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		ix := ctxBuildSearchIndex(cmd)
		model := semantic.Model(getStringFlag(cmd, "semantic-model", false))
		if model == "" {
			model = semantic.ModelBase8M
		}
		_, res, err := ctxBuildSemanticSnapshot(cmd, ix)
		exitWithError(cmd, err)
		out := ctxSnapshotResult{
			Path:     res.Path,
			Model:    string(model),
			Reused:   res.Reused,
			Embedded: res.Embedded,
		}
		if res.Snapshot != nil {
			out.Sections = len(res.Snapshot.SectionVectors)
		}
		switch getFormat(cmd) {
		case fmtJSON:
			b, err := json.MarshalIndent(out, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, b)
		case fmtYAML:
			b, err := yaml.Marshal(out)
			exitWithError(cmd, err)
			outputBytes(cmd, b)
		default:
			outputString(cmd, fmt.Sprintf(
				"snapshot %s (%s): %d sections (%d reused, %d embedded)\n",
				out.Path, out.Model, out.Sections, out.Reused, out.Embedded))
		}
	},
}

func init() {
	contextCmd.AddCommand(contextSnapshotCmd)
	contextSnapshotCmd.AddCommand(contextSnapshotBuildCmd)
	contextSnapshotBuildCmd.Flags().String("semantic-model", "", "Embedding model (default BASE8M)")
}
