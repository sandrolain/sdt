package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
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
	"dates": regexp.MustCompile(
		`\b(?:\d{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12]\d|3[01])|` +
			`(?:0[1-9]|[12]\d|3[01])[./](?:0[1-9]|1[0-2])[./]\d{4}|` +
			`(?:Jan(?:uary)?|Feb(?:ruary)?|Mar(?:ch)?|Apr(?:il)?|May|Jun(?:e)?|` +
			`Jul(?:y)?|Aug(?:ust)?|Sep(?:tember)?|Oct(?:ober)?|Nov(?:ember)?|Dec(?:ember)?)` +
			`\s+\d{1,2},?\s+\d{4})\b`,
	),
}

func extractJSONBlocks(input []byte) ([]string, error) {
	var matches []string
	for offset := 0; offset < len(input); {
		if input[offset] != '{' && input[offset] != '[' {
			offset++
			continue
		}

		start := offset
		decoder := json.NewDecoder(bytes.NewReader(input[start:]))
		decoder.UseNumber()
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("malformed JSON value at byte %d: %w", start, err)
		}
		end := start + int(decoder.InputOffset())
		matches = append(matches, string(bytes.TrimSpace(input[start:end])))
		offset = end
	}
	return matches, nil
}

func extractCodeBlocks(input []byte) ([]string, error) {
	root := parser.New().Parse(input)
	var matches []string
	err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		block, ok := node.(*ast.CodeBlock)
		if !ok || block.CodeBlockKind != ast.CodeBlockKindFenced || !codeBlockHasFenceCloser(input, block) {
			return ast.WalkContinue, nil
		}
		matches = append(matches, block.Value.Str(input))
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse Markdown code blocks: %w", err)
	}
	return matches, nil
}

var extractCmd = &cobra.Command{
	Use:   cmdExtract,
	Short: "Extract structured items from text input",
	Long: `Extract specific types of items from plain text using pattern matching and structural parsing.

Supported types: urls, emails, ips, json-blocks, code-blocks, dates.
JSON extraction returns complete outer object/array values and errors on malformed values.
Markdown extraction returns complete fenced block bodies without fence markers; unterminated fences are ignored.

Output is always a JSON array of strings.`,
	Run: func(cmd *cobra.Command, args []string) {
		extractType := getStringFlag(cmd, "type", false)
		if extractType == "" {
			exitWithError(cmd, fmt.Errorf("--type is required (urls|emails|ips|json-blocks|code-blocks|dates)"))
			return
		}

		if _, ok := extractPatterns[extractType]; !ok && extractType != "json-blocks" && extractType != "code-blocks" {
			exitWithError(cmd, fmt.Errorf("unknown type %q; supported: urls, emails, ips, json-blocks, code-blocks, dates", extractType))
			return
		}

		input := getInputString(cmd, args)
		var matches []string
		var err error
		switch extractType {
		case "json-blocks":
			matches, err = extractJSONBlocks([]byte(input))
		case "code-blocks":
			matches, err = extractCodeBlocks([]byte(input))
		default:
			matches = extractPatterns[extractType].FindAllString(input, -1)
		}
		exitWithError(cmd, err)
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
