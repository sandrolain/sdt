package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// promptCmd is the root command for prompt management.
var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Manage and render LLM prompt templates",
	Long: `Manage and render LLM prompt templates.

Subcommands:
  render    Render a prompt template with JSON/YAML variables
  validate  Validate a prompt against a maximum token budget
  tokens    Count tokens in a rendered prompt`,
}

// promptRenderCmd renders a Go text/template with variables and optionally
// reports the resulting token count.
var promptRenderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render a prompt template with variables",
	Long: `Render a Go text/template prompt with JSON or YAML variables.

Template source priority:
  1. --template flag (inline string)
  2. --file flag (path to template file)
  3. stdin

Variable source:
  1. --vars flag (inline JSON or YAML)
  2. --vars-file flag (path to JSON or YAML file)

Example:
  sdt prompt render --template "You are {{.role}}." --vars '{"role":"assistant"}'
  sdt prompt render --file system.txt --vars-file context.json`,
	Run: func(cmd *cobra.Command, args []string) {
		tmplStr := getStringFlag(cmd, "template", false)
		tmplFile := getStringFlag(cmd, "file", false)
		varsStr := getStringFlag(cmd, "vars", false)
		varsFile := getStringFlag(cmd, "vars-file", false)
		model := getStringFlag(cmd, "model", false)
		showTokens := getBoolFlag(cmd, "show-tokens", false)

		// Resolve template source
		switch {
		case tmplStr != "":
			// already set
		case tmplFile != "":
			b, err := os.ReadFile(tmplFile) //#nosec G304 -- user-controlled template file
			exitWithError(cmd, err)
			tmplStr = string(b)
		default:
			tmplStr = getInputString(cmd, args)
		}
		if tmplStr == "" {
			exitWithError(cmd, fmt.Errorf("template source required: use --template, --file, or pipe to stdin"))
			return
		}

		// Resolve variables
		var rawVars string
		switch {
		case varsStr != "":
			rawVars = varsStr
		case varsFile != "":
			b, err := os.ReadFile(varsFile) //#nosec G304 -- user-controlled vars file
			exitWithError(cmd, err)
			rawVars = string(b)
		}
		data, err := parseDataInput(rawVars)
		exitWithError(cmd, err)

		t, err := template.New("prompt").Parse(tmplStr)
		exitWithError(cmd, err)

		var buf bytes.Buffer
		exitWithError(cmd, t.Execute(&buf, data))
		rendered := buf.String()

		if showTokens {
			family := resolveModelFamily(model)
			count := CountTokens(rendered, family)
			type renderResult struct {
				Rendered string `json:"rendered" yaml:"rendered"`
				Tokens   int    `json:"tokens"   yaml:"tokens"`
				Model    string `json:"model"    yaml:"model"`
			}
			res := renderResult{Rendered: rendered, Tokens: count, Model: model}
			format := getFormat(cmd)
			switch format {
			case fmtYAML:
				out, merr := yaml.Marshal(res)
				exitWithError(cmd, merr)
				outputBytes(cmd, out)
			case fmtJSON:
				out, merr := json.MarshalIndent(res, "", "  ")
				exitWithError(cmd, merr)
				outputBytes(cmd, out)
			default:
				outputString(cmd, fmt.Sprintf("%s\n\n[tokens: %d (%s)]", rendered, count, model))
			}
			return
		}

		outputString(cmd, rendered)
	},
}

// promptValidateCmd validates a prompt template against a token budget.
var promptValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a prompt against a maximum token budget",
	Long: `Render a prompt template and check that it fits within a token budget.

Exits with code 1 if the rendered prompt exceeds --max-tokens.

Example:
  sdt prompt validate --file system.txt --max-tokens 4096 --model gpt-4
  echo "Tell me about {{.topic}}" | sdt prompt validate --vars '{"topic":"Go"}' --max-tokens 2000`,
	Run: func(cmd *cobra.Command, args []string) {
		tmplStr := getStringFlag(cmd, "template", false)
		tmplFile := getStringFlag(cmd, "file", false)
		varsStr := getStringFlag(cmd, "vars", false)
		varsFile := getStringFlag(cmd, "vars-file", false)
		model := getStringFlag(cmd, "model", false)
		maxTokens := getIntFlag(cmd, "max-tokens", false)

		// Resolve template source
		switch {
		case tmplStr != "":
		case tmplFile != "":
			b, err := os.ReadFile(tmplFile) //#nosec G304 -- user-controlled template file
			exitWithError(cmd, err)
			tmplStr = string(b)
		default:
			tmplStr = getInputString(cmd, args)
		}
		if tmplStr == "" {
			exitWithError(cmd, fmt.Errorf("template source required: use --template, --file, or pipe to stdin"))
			return
		}

		// Resolve variables
		var rawVars string
		switch {
		case varsStr != "":
			rawVars = varsStr
		case varsFile != "":
			b, err := os.ReadFile(varsFile) //#nosec G304 -- user-controlled vars file
			exitWithError(cmd, err)
			rawVars = string(b)
		}
		data, err := parseDataInput(rawVars)
		exitWithError(cmd, err)

		t, err := template.New("prompt").Parse(tmplStr)
		exitWithError(cmd, err)

		var buf bytes.Buffer
		exitWithError(cmd, t.Execute(&buf, data))
		rendered := buf.String()

		family := resolveModelFamily(model)
		count := CountTokens(rendered, family)

		type validResult struct {
			Valid     bool   `json:"valid"      yaml:"valid"`
			Tokens    int    `json:"tokens"     yaml:"tokens"`
			MaxTokens int    `json:"max_tokens" yaml:"max_tokens"`
			Model     string `json:"model"      yaml:"model"`
		}
		valid := maxTokens <= 0 || count <= maxTokens
		res := validResult{Valid: valid, Tokens: count, MaxTokens: maxTokens, Model: model}

		format := getFormat(cmd)
		switch format {
		case fmtYAML:
			out, merr := yaml.Marshal(res)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtJSON:
			out, merr := json.MarshalIndent(res, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			if valid {
				outputString(cmd, fmt.Sprintf("ok: %d tokens (limit %d)", count, maxTokens))
			} else {
				exitWithError(cmd, fmt.Errorf("exceeds token budget: %d tokens > %d limit", count, maxTokens))
			}
		}
		if !valid {
			exitWithError(cmd, fmt.Errorf("exceeds token budget: %d tokens > %d limit", count, maxTokens))
		}
	},
}

func init() {
	// prompt render flags
	promptRenderCmd.Flags().String("template", "", "Inline template string")
	promptRenderCmd.Flags().String("file", "", "Path to template file")
	promptRenderCmd.Flags().String("vars", "", "Variables as inline JSON or YAML")
	promptRenderCmd.Flags().String("vars-file", "", "Path to JSON or YAML variables file")
	promptRenderCmd.Flags().String("model", "gpt-4", "Model for token counting (used with --show-tokens)")
	promptRenderCmd.Flags().Bool("show-tokens", false, "Include token count in output")

	// prompt validate flags
	promptValidateCmd.Flags().String("template", "", "Inline template string")
	promptValidateCmd.Flags().String("file", "", "Path to template file")
	promptValidateCmd.Flags().String("vars", "", "Variables as inline JSON or YAML")
	promptValidateCmd.Flags().String("vars-file", "", "Path to JSON or YAML variables file")
	promptValidateCmd.Flags().String("model", "gpt-4", "Model for token counting")
	promptValidateCmd.Flags().Int("max-tokens", 4096, "Maximum allowed tokens")

	promptCmd.AddCommand(promptRenderCmd)
	promptCmd.AddCommand(promptValidateCmd)
	rootCmd.AddCommand(promptCmd)
}
