package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
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

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Render a Go text/template with JSON or YAML data",
	Long: `Render a Go text/template using JSON or YAML data.

Template source (in priority order):
  1. --tmpl flag
  2. stdin (when --data or --file-data is provided)

Data source (in priority order):
  1. --data flag (inline JSON or YAML string)
  2. --file-data flag (path to JSON or YAML file)
  3. No data (template receives nil)

Example:
  echo '{"name":"Alice"}' | sdt template --tmpl "Hello, {{.name}}!"`,
	Run: func(cmd *cobra.Command, args []string) {
		tmplStr := getStringFlag(cmd, "tmpl", false)
		dataStr := getStringFlag(cmd, "data", false)
		fileData := getStringFlag(cmd, "file-data", false)

		// Resolve template source
		if tmplStr == "" {
			tmplStr = getInputString(cmd, args)
		}
		if tmplStr == "" {
			exitWithError(cmd, fmt.Errorf("template source required: use --tmpl or pipe to stdin"))
			return
		}

		// Resolve data source
		var rawData string
		switch {
		case dataStr != "":
			rawData = dataStr
		case fileData != "":
			b, err := os.ReadFile(fileData) //#nosec G304 -- user-controlled data file
			exitWithError(cmd, err)
			rawData = string(b)
		}

		data, err := parseDataInput(rawData)
		exitWithError(cmd, err)

		// Parse and execute template
		t, err := template.New("sdt").Parse(tmplStr)
		exitWithError(cmd, err)

		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			exitWithError(cmd, err)
		}
		outputBytes(cmd, buf.Bytes())
	},
}

func init() {
	templateCmd.Flags().String("tmpl", "", "Template string (Go text/template syntax)")
	templateCmd.Flags().String("data", "", "Inline data as JSON or YAML")
	templateCmd.Flags().String("file-data", "", "Path to JSON or YAML data file")
	rootCmd.AddCommand(templateCmd)
}
