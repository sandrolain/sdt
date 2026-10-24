package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── comprehension sample (sdt context wiki comprehension) ───────────────────
//
// The sample is a review aid, not a measurement: two pages per book, one dense
// and one at the boundary, so a reader cannot pass by only sampling the pages
// that already have many claims.

type comprehensionSample struct {
	Book           string `json:"book" yaml:"book"`
	Artifact       string `json:"artifact" yaml:"artifact"`
	Dense          string `json:"dense" yaml:"dense"`
	DenseClaims    int    `json:"dense_claims" yaml:"dense_claims"`
	Boundary       string `json:"boundary,omitempty" yaml:"boundary,omitempty"`
	BoundaryClaims int    `json:"boundary_claims,omitempty" yaml:"boundary_claims,omitempty"`
	Prompt         string `json:"prompt" yaml:"prompt"`
}

const comprehensionPrompt = "For each page, state its claim and why it matters without opening the source; record pass, partial or fail."

var contextWikiComprehensionCmd = &cobra.Command{
	Use:   "comprehension [--book <index>]",
	Short: "Select the two-page-per-book comprehension sample",
	Long: `Select a deterministic six-page sample: two concept pages per book, one
with the most claims (dense) and one with the fewest (boundary). The command
never records a verdict; the receipt records pass/partial/fail with the ids.

With --book (repeatable) only those index pages are sampled; without it every
entity page carrying a refs/ source is used.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		bld, err := loadWikiForComprehension()
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		books, err := comprehensionBooks(cmd, bld)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		samples := comprehensionSamples(bld, books)
		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(samples, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(samples)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			for _, s := range samples {
				outputString(cmd, fmt.Sprintf("book: %s\nartifact: %s\ndense: %s (claims=%d)\n", s.Book, s.Artifact, s.Dense, s.DenseClaims))
				if s.Boundary != "" {
					outputString(cmd, fmt.Sprintf("boundary: %s (claims=%d)\n", s.Boundary, s.BoundaryClaims))
				} else {
					outputString(cmd, "boundary: -\n")
				}
				outputString(cmd, fmt.Sprintf("prompt: %s\n", s.Prompt))
			}
		}
	},
}

func loadWikiForComprehension() (*contextwiki.Builder, error) {
	if _, err := os.Stat(sdtWikiDir); os.IsNotExist(err) {
		return &contextwiki.Builder{}, nil
	}
	bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
	if err != nil {
		return nil, fmt.Errorf("load wiki: %w", err)
	}
	return bld, nil
}

func comprehensionBooks(cmd *cobra.Command, bld *contextwiki.Builder) ([]*contextwiki.Page, error) {
	refs := getStringArrayFlag(cmd, "book", false)
	if len(refs) == 0 {
		var books []*contextwiki.Page
		for _, page := range bld.Pages {
			if page.Type == wikiTypeEntity && artifactSourceForPage(page) != "" {
				books = append(books, page)
			}
		}
		sort.Slice(books, func(i, j int) bool { return books[i].ID < books[j].ID })
		return books, nil
	}
	var books []*contextwiki.Page
	for _, ref := range refs {
		path, err := resolveWikiPage(ref)
		if err != nil {
			return nil, err
		}
		id := strings.TrimPrefix(strings.TrimSuffix(corpusRelative(path), ".md"), "wiki/")
		page := bld.Resolve(id)
		if page == nil {
			return nil, fmt.Errorf("book %q is not in the wiki", ref)
		}
		books = append(books, page)
	}
	return books, nil
}

func artifactSourceForPage(page *contextwiki.Page) string {
	for _, source := range page.Sources {
		if strings.HasPrefix(source, "refs/") {
			return source
		}
	}
	return ""
}

func comprehensionSamples(bld *contextwiki.Builder, books []*contextwiki.Page) []comprehensionSample {
	var samples []comprehensionSample
	for _, book := range books {
		artifact := artifactSourceForPage(book)
		if artifact == "" {
			continue
		}
		var pages []*contextwiki.Page
		for _, page := range bld.Pages {
			if page.Type == "concept" && inventoryContainsString(page.Sources, artifact) {
				pages = append(pages, page)
			}
		}
		if len(pages) == 0 {
			continue
		}
		sort.Slice(pages, func(i, j int) bool {
			if pages[i].ClaimCount != pages[j].ClaimCount {
				return pages[i].ClaimCount > pages[j].ClaimCount
			}
			return pages[i].ID < pages[j].ID
		})
		sample := comprehensionSample{
			Book:        book.ID,
			Artifact:    artifact,
			Dense:       pages[0].ID,
			DenseClaims: pages[0].ClaimCount,
			Prompt:      comprehensionPrompt,
		}
		if len(pages) > 1 {
			last := pages[len(pages)-1]
			sample.Boundary = last.ID
			sample.BoundaryClaims = last.ClaimCount
		}
		samples = append(samples, sample)
	}
	return samples
}

func init() {
	contextWikiComprehensionCmd.Flags().StringArray("book", nil, "Index page to sample (repeatable; default: every entity page with a refs/ source)")
	contextWikiCmd.AddCommand(contextWikiComprehensionCmd)
}
