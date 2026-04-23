package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
)

// extractPatterns maps type names to compiled regexes.
var extractPatterns = map[string]*regexp.Regexp{
	"urls": regexp.MustCompile(
		`https?://[^\s<>"']+`,
	),
	"emails": regexp.MustCompile(
		`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`,
	),
	"ips": regexp.MustCompile(
		`\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`,
	),
	"json-blocks": regexp.MustCompile(
		`(?s)\{[^{}]*(?:\{[^{}]*\}[^{}]*)?\}|\[[^\[\]]*(?:\[[^\[\]]*\][^\[\]]*)*\]`,
	),
	"code-blocks": regexp.MustCompile(
		"(?s)```[a-zA-Z]*\\n[\\s\\S]*?```|`[^`\\n]+`",
	),
	"dates": regexp.MustCompile(
		`\b(?:\d{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12]\d|3[01])|` +
			`(?:0[1-9]|[12]\d|3[01])[./](?:0[1-9]|1[0-2])[./]\d{4}|` +
			`(?:Jan(?:uary)?|Feb(?:ruary)?|Mar(?:ch)?|Apr(?:il)?|May|Jun(?:e)?|` +
			`Jul(?:y)?|Aug(?:ust)?|Sep(?:tember)?|Oct(?:ober)?|Nov(?:ember)?|Dec(?:ember)?)` +
			`\s+\d{1,2},?\s+\d{4})\b`,
	),
}

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract structured items from text input",
	Long: `Extract specific types of items from plain text using pattern matching.

Supported types: urls, emails, ips, json-blocks, code-blocks, dates

Output is always a JSON array of strings.`,
	Run: func(cmd *cobra.Command, args []string) {
		extractType := getStringFlag(cmd, "type", false)
		if extractType == "" {
			exitWithError(cmd, fmt.Errorf("--type is required (urls|emails|ips|json-blocks|code-blocks|dates)"))
			return
		}

		re, ok := extractPatterns[extractType]
		if !ok {
			exitWithError(cmd, fmt.Errorf("unknown type %q; supported: urls, emails, ips, json-blocks, code-blocks, dates", extractType))
			return
		}

		input := getInputString(cmd, args)
		matches := re.FindAllString(input, -1)
		if matches == nil {
			matches = []string{}
		}

		out, err := json.MarshalIndent(matches, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	},
}

func init() {
	extractCmd.Flags().String("type", "", "Type of items to extract (urls|emails|ips|json-blocks|code-blocks|dates)")
	rootCmd.AddCommand(extractCmd)
}
