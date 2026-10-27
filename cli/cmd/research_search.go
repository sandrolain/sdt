package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/sandrolain/sdt/internal/research"
	"github.com/spf13/cobra"
)

// searchProviderSetFactory is overridable in tests so the command never touches the
// network under `go test`. It reads the project's `research:` order, falls back to the
// default one, and prepends an ad-hoc Firecrawl entry when --api-key/--api-url are given,
// so a one-off run behaves as it always did without a config file.
var searchProviderSetFactory = func(cmd *cobra.Command) (*research.ProviderSet, []research.Attempt, error) {
	cfgs := research.DefaultProviders()
	if cfg, err := findProjectConfig(); err == nil && cfg != nil && len(cfg.Research) > 0 {
		cfgs = make([]research.ProviderConfig, 0, len(cfg.Research))
		for _, entry := range cfg.Research {
			cfgs = append(cfgs, research.ProviderConfig{
				Provider: entry.Provider,
				Endpoint: entry.Endpoint,
				Key:      entry.Key,
				KeyEnv:   entry.KeyEnv,
				KeyFile:  entry.KeyFile,
			})
		}
	}
	apiKey, apiURL := getStringFlag(cmd, "api-key", false), getStringFlag(cmd, "api-url", false)
	if apiKey != "" || apiURL != "" {
		cfgs = append([]research.ProviderConfig{{Provider: "firecrawl", Key: apiKey, Endpoint: apiURL}}, cfgs...)
	}
	return research.NewProviderSet(cfgs, os.Getenv)
}

var researchSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Discover sources for a query and record them in the run",
	Long: `Discover sources for a query with the project's declared providers, tried in
the order .sdt.yaml gives them, and record them as 'discovered' in the run
manifest. Search is opt-in: it refuses to run without --network and --confirm,
because it reaches the network and consumes provider credits.

Only discovery happens here; content acquisition stays with 'sdt crawldown'.

No research: block means tavily, firecrawl, exa, searxng. A paid provider whose
key is not set is skipped and recorded; SearXNG needs a declared endpoint. Keys
come from the entry (key, key_env, key_file), then the shell or the project .env.
--api-key/--api-url add an ad-hoc Firecrawl entry before the declared order.
--limit caps the results and --max-credits aborts before starting when the first
provider's remaining credits are below it.

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

		providers, _, err := searchProviderSetFactory(cmd)
		exitWithError(cmd, err)

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Pre-flight credit check: abort before spending when below the budget. Only the
		// provider a run would spend first is asked; the others report unknown (-1).
		remaining, cerr := providers.CreditUsage(ctx)
		if cerr == nil && maxCredits > 0 && remaining >= 0 && remaining < maxCredits {
			exitWithError(cmd, fmt.Errorf("remaining credits (%d) are below --max-credits (%d): refusing to start", remaining, maxCredits))
			return
		}

		results, attempts, serr := providers.Search(ctx, query, limit)
		if serr != nil {
			exitWithError(cmd, serr)
			return
		}
		r.Provider, r.ProviderAttempts = "", attempts
		for _, a := range attempts {
			if a.OK {
				r.Provider = a.Provider
			}
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
			if after, aerr := providers.CreditUsage(ctx); aerr == nil && after >= 0 {
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
