package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/sandrolain/sdt/internal/research"
	"github.com/spf13/cobra"
)

// searchProviderFactory is overridable in tests so the command never touches
// the network under `go test`.
var searchProviderFactory = func(cmd *cobra.Command) (research.SearchProvider, error) {
	apiKey := getStringFlag(cmd, "api-key", false)
	apiURL := getStringFlag(cmd, "api-url", false)
	return research.NewFirecrawlProvider(apiKey, apiURL)
}

var researchSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Discover sources for a query and record them in the run",
	Long: `Discover sources for a query with the configured provider (Firecrawl) and
record them as 'discovered' in the run manifest. Search is opt-in: it refuses
to run without --network and --confirm, because it reaches the network and
consumes provider credits.

Only discovery happens here; content acquisition stays with 'sdt crawldown'.

The API key comes from FIRECRAWL_API_KEY (shell or project .env) or --api-key;
--api-url points the SDK at a self-hosted instance. --limit caps the number of
results and --max-credits aborts before starting when the remaining credits are
below it.

Examples:
  sdt research init --query "vector stores"
  sdt research search --query "embedded vector store Go" --limit 5 --network --confirm
  sdt research search --run 01a0f… --query "…" --max-credits 10 --network --confirm`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if !getBoolFlag(cmd, "network", false) || !getBoolFlag(cmd, "confirm", false) {
			exitWithError(cmd, errors.New("search reaches the network and spends credits: pass --network and --confirm to proceed"))
			return
		}

		r, root, err := resolveRun(cmd)
		exitWithError(cmd, err)

		query := getStringFlag(cmd, "query", false)
		if query == "" {
			query = r.Query
		}
		if query == "" {
			exitWithError(cmd, errors.New("no query: pass --query or create the run with one"))
			return
		}

		limit := getIntFlag(cmd, "limit", false)
		maxCredits := getIntFlag(cmd, "max-credits", false)

		provider, err := searchProviderFactory(cmd)
		exitWithError(cmd, err)

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Pre-flight credit check: abort before spending when below the budget.
		remaining, cerr := provider.CreditUsage(ctx)
		if cerr == nil && maxCredits > 0 && remaining >= 0 && remaining < maxCredits {
			exitWithError(cmd, fmt.Errorf("remaining credits (%d) are below --max-credits (%d): refusing to start", remaining, maxCredits))
			return
		}

		results, serr := provider.Search(ctx, query, limit)
		if serr != nil {
			exitWithError(cmd, serr)
			return
		}
		if limit > 0 && len(results) > limit {
			results = results[:limit]
		}

		added := 0
		for _, res := range results {
			if res.URL == "" {
				continue
			}
			if r.AddSource(research.Source{
				CanonicalURL: res.URL,
				Title:        res.Title,
				Snippet:      res.Snippet,
				Status:       research.StatusDiscovered,
			}) {
				added++
			}
		}

		r.Budget.MaxResults = limit
		r.Budget.MaxCredits = maxCredits
		r.Budget.Results = len(r.Sources)
		if remaining >= 0 {
			if after, aerr := provider.CreditUsage(ctx); aerr == nil && after >= 0 {
				r.Budget.CreditsUsed += creditDelta(remaining, after)
			}
		}
		r.MarkCheckpoint("search")
		if err := r.Save(root); err != nil {
			exitWithError(cmd, err)
		}

		summary := fmt.Sprintf("run %s: %d result(s), %d new source(s)\n", r.RunID, len(results), added)
		outputResearch(cmd, r, func() string { return summary })
	},
}

// creditDelta returns how many credits were consumed between two readings. A
// top-up between the readings yields 0 rather than a negative number.
func creditDelta(before, after int) int {
	if after >= before {
		return 0
	}
	return before - after
}

func init() {
	researchSearchCmd.Flags().String("query", "", "Search query (default: the run query)")
	researchSearchCmd.Flags().Int("limit", 10, "Maximum number of results")
	researchSearchCmd.Flags().Int("max-credits", 0, "Abort when remaining credits are below this (0 = no check)")
	researchSearchCmd.Flags().String("api-key", "", "Firecrawl API key (default: FIRECRAWL_API_KEY)")
	researchSearchCmd.Flags().String("api-url", "", "Firecrawl API base URL (default: hosted endpoint)")
	researchSearchCmd.Flags().Bool("network", false, "Allow the command to reach the network")
	researchSearchCmd.Flags().Bool("confirm", false, "Confirm the run may spend provider credits")
	researchSearchCmd.Flags().String("run", "", "Run id (default: the latest run)")
	researchCmd.AddCommand(researchSearchCmd)
}
