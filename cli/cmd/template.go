package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// parseDataInput unmarshals a JSON or YAML string into a map.
func parseDataInput(s string) (map[string]interface{}, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var data map[string]interface{}
	// Try JSON first
	if err := json.Unmarshal([]byte(s), &data); err == nil {
		return data, nil
	}
	// Fall back to YAML
	if err := yaml.Unmarshal([]byte(s), &data); err == nil {
		return data, nil
	}
	return nil, fmt.Errorf("data is neither valid JSON nor valid YAML")
}

func templateFlagChanged(cmd *cobra.Command, name string) bool {
	flag := cmd.Flags().Lookup(name)
	return flag != nil && flag.Changed
}

func templateSource(cmd *cobra.Command, args []string) (string, error) {
	sources := []string{"tmpl", "template-file", "file", "input", "inb64"}
	count := 0
	for _, name := range sources {
		if templateFlagChanged(cmd, name) {
			count++
		}
	}
	if len(args) > 0 {
		count++
	}
	if count > 1 {
		return "", fmt.Errorf("choose one template source: --tmpl, --template-file, --file, --input, --inb64, positional input, or stdin")
	}
	if count == 0 {
		return getInputString(cmd, args), nil
	}
	hasStdin, err := templateStdinHasInput(cmd.InOrStdin())
	if err != nil {
		return "", err
	}
	if hasStdin {
		return "", fmt.Errorf("stdin and an explicit template source were both provided")
	}

	if templateFlagChanged(cmd, "template-file") {
		path := getStringFlag(cmd, "template-file", true)
		content, err := os.ReadFile(path) //#nosec G304 -- user-controlled template file
		if err != nil {
			return "", fmt.Errorf("read template file %q: %w", path, err)
		}
		return string(content), nil
	}
	if templateFlagChanged(cmd, "tmpl") {
		return getStringFlag(cmd, "tmpl", false), nil
	}
	return getInputString(cmd, args), nil
}

func templateStdinHasInput(input io.Reader) (bool, error) {
	if file, ok := input.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return false, fmt.Errorf("inspect template stdin: %w", err)
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			return false, nil
		}
	}
	data, err := io.ReadAll(input)
	if err != nil {
		return false, fmt.Errorf("read template stdin: %w", err)
	}
	return len(data) > 0, nil
}

func renderTemplate(cmd *cobra.Command, args []string) (string, error) {
	tmplStr, err := templateSource(cmd, args)
	if err != nil {
		return "", err
	}
	if tmplStr == "" {
		return "", fmt.Errorf("template source required")
	}

	dataStr := getStringFlag(cmd, "data", false)
	fileData := getStringFlag(cmd, "file-data", false)
	if templateFlagChanged(cmd, "data") && templateFlagChanged(cmd, "file-data") || dataStr != "" && fileData != "" {
		return "", fmt.Errorf("choose one variable source: --data or --file-data")
	}
	if templateFlagChanged(cmd, "file-data") && strings.TrimSpace(fileData) == "" {
		return "", fmt.Errorf("--file-data requires a file path")
	}

	var rawData string
	switch {
	case dataStr != "":
		rawData = dataStr
	case fileData != "":
		content, err := os.ReadFile(fileData) //#nosec G304 -- user-controlled data file
		if err != nil {
			return "", fmt.Errorf("read data file %q: %w", fileData, err)
		}
		rawData = string(content)
	}
	data, err := parseDataInput(rawData)
	if err != nil {
		return "", err
	}

	t, err := template.New("sdt").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var output bytes.Buffer
	if err := t.Execute(&output, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	return output.String(), nil
}

// TemplateValidationResult reports an approximate rendered-template budget check.
type TemplateValidationResult struct {
	Valid           bool   `json:"valid"            yaml:"valid"`
	EstimatedTokens int    `json:"estimated_tokens" yaml:"estimated_tokens"`
	MaxTokens       int    `json:"max_tokens"       yaml:"max_tokens"`
	Model           string `json:"model"            yaml:"model"`
	TokenizerFamily string `json:"tokenizer_family" yaml:"tokenizer_family"`
	Note            string `json:"note"              yaml:"note"`
}

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Render a Go text/template with JSON or YAML data",
	Long: `Render a Go text/template using JSON or YAML data.

Choose one template source: --tmpl, --template-file, --file, --input, --inb64,
positional input, or stdin.

Choose at most one variable source: --data (inline JSON or YAML) or
--file-data (a JSON or YAML file). Stdin is the template source, never a
variable source.

Example:
  sdt template --template-file greeting.tmpl --file-data vars.yaml`,
	Run: func(cmd *cobra.Command, args []string) {
		rendered, err := renderTemplate(cmd, args)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		outputString(cmd, rendered)
	},
}

var templateValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Check a rendered template against an estimated token limit",
	Long: `Render the selected template and data, then compare the approximate token
count with --max-tokens. JSON and YAML output include the validation result and
the estimate limitation. Exits with code 1 when the estimate exceeds the limit.`,
	Run: func(cmd *cobra.Command, args []string) {
		rendered, err := renderTemplate(cmd, args)
		if err != nil {
			exitWithError(cmd, err)
			return
		}

		model := getStringFlag(cmd, "model", false)
		if model == "" {
			model = defaultModel
		}
		maxTokens := getIntFlag(cmd, "max-tokens", false)
		family := resolveModelFamily(model)
		estimated := CountTokens(rendered, family)
		valid := maxTokens <= 0 || estimated <= maxTokens
		note := tokenEstimateNote(model)
		if !valid {
			note = fmt.Sprintf("estimated token budget exceeded; %s", note)
		}
		result := TemplateValidationResult{
			Valid:           valid,
			EstimatedTokens: estimated,
			MaxTokens:       maxTokens,
			Model:           model,
			TokenizerFamily: family,
			Note:            note,
		}

		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(result, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(result)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			if valid {
				outputString(cmd, fmt.Sprintf("ok: estimated %d tokens of %d (%s)", estimated, maxTokens, note))
			} else {
				exitWithError(cmd, fmt.Errorf("exceeds estimated token budget: %d tokens > %d limit (%s)", estimated, maxTokens, note))
				return
			}
		}
		if !valid {
			exitWithError(cmd, fmt.Errorf("exceeds estimated token budget: %d tokens > %d limit (%s)", estimated, maxTokens, note))
		}
	},
}

func init() {
	templateCmd.PersistentFlags().String("tmpl", "", "Template string (Go text/template syntax)")
	templateCmd.PersistentFlags().String("template-file", "", "Path to a Go text/template file")
	templateCmd.PersistentFlags().String("data", "", "Inline data as JSON or YAML")
	templateCmd.PersistentFlags().String("file-data", "", "Path to JSON or YAML data file")
	templateValidateCmd.Flags().Int("max-tokens", 4096, "Maximum estimated tokens for the rendered template")
	templateValidateCmd.Flags().String("model", defaultModel, "Model used to select a token estimator")
	templateCmd.AddCommand(templateValidateCmd)
	rootCmd.AddCommand(templateCmd)
}
