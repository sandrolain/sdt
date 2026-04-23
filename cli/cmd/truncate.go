package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// sentenceEndRe matches sentence-ending punctuation followed by whitespace.
var sentenceEndRe = regexp.MustCompile(`[.!?]+[\s]+`)

// markdownSectionRe matches a markdown heading line.
var markdownSectionRe = regexp.MustCompile(`(?m)^#{1,6}\s+.+$`)

// truncateToTokens cuts the text so that CountTokens(result, family) <= maxTokens.
// It uses a binary-search approach over rune offsets for efficiency.
func truncateToTokens(text, family string, maxTokens int) string {
	if CountTokens(text, family) <= maxTokens {
		return text
	}
	runes := []rune(text)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if CountTokens(string(runes[:mid]), family) <= maxTokens {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo])
}

// truncateSentences truncates text at a sentence boundary so the result fits
// within maxTokens. It prefers ending at a complete sentence.
func truncateSentences(text, family string, maxTokens int) string {
	raw := truncateToTokens(text, family, maxTokens)
	if raw == text {
		return text
	}
	// Find the last sentence boundary within raw.
	locs := sentenceEndRe.FindAllStringIndex(raw, -1)
	if len(locs) == 0 {
		return raw
	}
	lastEnd := locs[len(locs)-1][1]
	return strings.TrimRight(raw[:lastEnd], " \t")
}

// truncateSections truncates text at a markdown section boundary so the result
// fits within maxTokens. It keeps whole sections.
func truncateSections(text, family string, maxTokens int) string {
	// Split on heading lines preserving them.
	locs := markdownSectionRe.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		// Not markdown — fall back to sentence strategy.
		return truncateSentences(text, family, maxTokens)
	}

	// Build section start offsets.
	starts := make([]int, 0, len(locs)+1)
	starts = append(starts, 0)
	for _, loc := range locs {
		if loc[0] > 0 {
			starts = append(starts, loc[0])
		}
	}

	// Keep adding sections as long as they fit.
	best := ""
	for _, start := range starts {
		// Include text from this heading up to the next heading (or end).
		nextStart := len(text)
		for _, s := range starts {
			if s > start {
				nextStart = s
				break
			}
		}
		candidate := text[:nextStart]
		if CountTokens(candidate, family) <= maxTokens {
			best = candidate
		} else {
			break
		}
	}
	if best == "" {
		return truncateSentences(text, family, maxTokens)
	}
	return strings.TrimRight(best, "\n")
}

var truncateCmd = &cobra.Command{
	Use:   "truncate",
	Short: "Truncate text to a maximum number of LLM tokens",
	Long: `Truncate input text so that it fits within a maximum token budget.

Strategies:
  chars       Hard cut at character level (default)
  sentences   Cut at the last complete sentence boundary
  sections    Cut at the last complete markdown section boundary

Example:
  cat long_doc.md | sdt truncate --max-tokens 4000
  sdt truncate --max-tokens 2000 --strategy sentences --file essay.txt
  sdt truncate --max-tokens 1000 --strategy sections --model claude --file README.md`,
	Run: func(cmd *cobra.Command, args []string) {
		maxTokens := getIntFlag(cmd, "max-tokens", false)
		strategy := getStringFlag(cmd, "strategy", false)
		model := getStringFlag(cmd, "model", false)

		if maxTokens <= 0 {
			exitWithError(cmd, fmt.Errorf("--max-tokens must be a positive integer"))
			return
		}

		family := resolveModelFamily(model)
		text := getInputString(cmd, args)

		var result string
		switch strategy {
		case "sentences":
			result = truncateSentences(text, family, maxTokens)
		case "sections":
			result = truncateSections(text, family, maxTokens)
		default: // "chars" or empty
			result = truncateToTokens(text, family, maxTokens)
		}

		outputString(cmd, result)
	},
}

func init() {
	truncateCmd.Flags().Int("max-tokens", 4096, "Maximum number of tokens to keep")
	truncateCmd.Flags().String("strategy", "chars", "Truncation strategy: chars|sentences|sections")
	truncateCmd.Flags().String("model", "gpt-4", "Model name for tokenizer selection")
	rootCmd.AddCommand(truncateCmd)
}
