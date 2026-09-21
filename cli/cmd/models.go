package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/semantic"
)

// modelsCmd is the `sdt models` group: it manages the static embedding models
// used by the optional semantic search branch. Models are downloaded once into
// the user cache (os.UserCacheDir()/go-potion), never bundled in the binary.
var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage local models for semantic search",
}

var modelsFetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Download and cache a static embedding model",
	Long: `Download a static embedding model into the local cache
(os.UserCacheDir()/go-potion) so semantic search works offline afterwards.

Models (size grows with quality): BASE2M (~8 MB, 64 dims), BASE8M (~31 MB,
256 dims, default), MULTILINGUAL128M (~506 MB, 256 dims, 101 languages).

Examples:
  sdt models fetch
  sdt models fetch --model MULTILINGUAL128M
  sdt context search "hybrid search" --semantic`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		model := semantic.Model(getStringFlag(cmd, "model", false))
		if model == "" {
			model = semantic.ModelBase8M
		}
		ix, err := semantic.New(context.Background(), model)
		if err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, fmt.Sprintf("model %s ready (count=%d)\n", ix.Model(), ix.Count()))
	},
}

func init() {
	modelsFetchCmd.Flags().String("model", "", "Model id: BASE2M|BASE8M|MULTILINGUAL128M (default BASE8M)")
	modelsCmd.AddCommand(modelsFetchCmd)
	rootCmd.AddCommand(modelsCmd)
}
