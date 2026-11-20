package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// sentenceEndRe matches sentence-ending punctuation followed by whitespace.
var sentenceEndRe = regexp.MustCompile(`[.!?]+[\s]+`)

// markdownSectionRe matches a markdown heading line.
var markdownSectionRe = regexp.MustCompile(`(?m)^#{1,6}\s+.+$`)

// TruncateResult reports the retained text and its approximate budget metadata.
type TruncateResult struct {
	Text              string `json:"text" yaml:"text"`
	Model             string `json:"model" yaml:"model"`
	TokenizerFamily   string `json:"tokenizer_family" yaml:"tokenizer_family"`
	EstimatedTokens   int    `json:"estimated_tokens" yaml:"estimated_tokens"`
	MaxTokens         int    `json:"max_tokens" yaml:"max_tokens"`
	Truncated         bool   `json:"truncated" yaml:"truncated"`
	OmittedCharacters int    `json:"omitted_characters" yaml:"omitted_characters"`
	Warning           string `json:"warning" yaml:"warning"`
}

// truncateToTokens cuts text until its approximate token count is at most maxTokens.
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
	if CountTokens(text, family) <= maxTokens {
		return text
	}
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
	Short: "Truncate text using an approximate token limit",
	Long: `Truncate input text until the selected tokenizer approximation is at or
below --max-tokens. The count is only an estimate and is not provider-exact.
Text output stays on stdout with the estimate and omission warning on stderr.
JSON and YAML output include the text, estimate, truncation state, omitted
character count, and warning as structured fields.

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

		omittedChars := utf8.RuneCountInString(text) - utf8.RuneCountInString(result)
		truncated := result != text
		warning := tokenEstimateNote(model)
		if truncated {
			warning = fmt.Sprintf("input was truncated; %s; omitted %d characters", warning, omittedChars)
		}
		metadata := TruncateResult{
			Text:              result,
			Model:             model,
			TokenizerFamily:   family,
			EstimatedTokens:   CountTokens(result, family),
			MaxTokens:         maxTokens,
			Truncated:         truncated,
			OmittedCharacters: omittedChars,
			Warning:           warning,
		}

		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(metadata, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(metadata)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			outputString(cmd, result)
			cmd.PrintErrf("%s; estimated retained tokens: %d of %d; truncated: %t; omitted characters: %d\n",
				warning, metadata.EstimatedTokens, maxTokens, truncated, omittedChars)
		}
	},
}

func init() {
	truncateCmd.Flags().Int("max-tokens", 4096, "Maximum number of tokens to keep")
	truncateCmd.Flags().String("strategy", "chars", "Truncation strategy: chars|sentences|sections")
	truncateCmd.Flags().String("model", "gpt-4", "Model name for tokenizer selection")
	rootCmd.AddCommand(truncateCmd)
}
