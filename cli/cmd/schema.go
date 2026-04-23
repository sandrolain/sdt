package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// JSONSchema represents a minimal JSON Schema (draft-07) document.
type JSONSchema struct {
	Schema      string                 `json:"$schema,omitempty"      yaml:"$schema,omitempty"`
	Title       string                 `json:"title,omitempty"        yaml:"title,omitempty"`
	Description string                 `json:"description,omitempty"  yaml:"description,omitempty"`
	Type        string                 `json:"type,omitempty"         yaml:"type,omitempty"`
	Properties  map[string]*JSONSchema `json:"properties,omitempty"   yaml:"properties,omitempty"`
	Items       *JSONSchema            `json:"items,omitempty"        yaml:"items,omitempty"`
	Enum        []string               `json:"enum,omitempty"         yaml:"enum,omitempty"`
	Default     interface{}            `json:"default,omitempty"      yaml:"default,omitempty"`
	Required    []string               `json:"required,omitempty"     yaml:"required,omitempty"`
}

// CommandSchema wraps a command's JSON Schema.
type CommandSchema struct {
	Command     string     `json:"command"               yaml:"command"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	Input       *JSONSchema `json:"input"                yaml:"input"`
	Flags       *JSONSchema `json:"flags"                yaml:"flags"`
}

// cobraFlagTypeToJSONType maps pflag type names to JSON Schema types.
func cobraFlagTypeToJSONType(t string) string {
	switch t {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	case "stringArray", "stringSlice":
		return "array"
	default:
		return "string"
	}
}

// buildFlagSchema builds a JSON Schema describing a command's flags.
func buildFlagSchema(c *cobra.Command) *JSONSchema {
	props := map[string]*JSONSchema{}
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		s := &JSONSchema{
			Description: f.Usage,
			Type:        cobraFlagTypeToJSONType(f.Value.Type()),
		}
		if f.DefValue != "" && f.DefValue != "[]" && f.DefValue != "false" {
			s.Default = f.DefValue
		}
		if f.Value.Type() == "stringArray" || f.Value.Type() == "stringSlice" {
			s.Items = &JSONSchema{Type: "string"}
		}
		props[f.Name] = s
	})
	if len(props) == 0 {
		return &JSONSchema{Type: "object"}
	}
	return &JSONSchema{Type: "object", Properties: props}
}

// buildCommandSchema creates a CommandSchema for a cobra.Command.
func buildCommandSchema(c *cobra.Command) CommandSchema {
	return CommandSchema{
		Command:     strings.Join(getUseArray(c), " "),
		Description: c.Short,
		Input: &JSONSchema{
			Schema:      "http://json-schema.org/draft-07/schema#",
			Type:        "string",
			Description: "Input text. Provide via stdin, --input, --file, or --inb64.",
		},
		Flags: buildFlagSchema(c),
	}
}

// findCommand locates a cobra command by its full use path (e.g. "jwt parse").
func findCommand(root *cobra.Command, path string) (*cobra.Command, bool) {
	parts := strings.Fields(path)
	if len(parts) == 0 {
		return nil, false
	}
	// Skip the root command name if provided
	current := root
	for _, part := range parts {
		found := false
		for _, sub := range current.Commands() {
			if sub.Name() == part {
				current = sub
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return current, true
}

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Generate JSON Schema for SDT commands",
	Long: `Generate JSON Schema documents describing SDT command inputs and flags.

Without --command, emits a schema for every command as a JSON array.
With --command, emits the schema for a single command.

Examples:
  sdt schema                          # all commands
  sdt schema --command "jwt parse"    # single command
  sdt schema --format yaml            # YAML output`,
	Run: func(cmd *cobra.Command, args []string) {
		cmdPath := getStringFlag(cmd, "command", false)
		root := cmd.Root()
		format := getFormat(cmd)

		if cmdPath != "" {
			target, ok := findCommand(root, cmdPath)
			if !ok {
				exitWithError(cmd, fmt.Errorf("command %q not found", cmdPath))
				return
			}
			schema := buildCommandSchema(target)
			emitSchemaOutput(cmd, schema, format)
			return
		}

		// All runnable leaf commands (not groups, not hidden).
		type allSchema struct {
			Schema   string          `json:"$schema" yaml:"$schema"`
			Commands []CommandSchema `json:"commands" yaml:"commands"`
		}
		var schemas []CommandSchema
		collectSchemas(root, &schemas)

		result := allSchema{
			Schema:   "http://json-schema.org/draft-07/schema#",
			Commands: schemas,
		}
		switch format {
		case fmtYAML:
			out, err := yaml.Marshal(result)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			out, err := json.MarshalIndent(result, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		}
	},
}

func collectSchemas(c *cobra.Command, out *[]CommandSchema) {
	if c.Hidden {
		return
	}
	subs := c.Commands()
	if len(subs) == 0 && c.Run != nil {
		*out = append(*out, buildCommandSchema(c))
		return
	}
	for _, sub := range subs {
		collectSchemas(sub, out)
	}
}

func emitSchemaOutput(cmd *cobra.Command, schema CommandSchema, format string) {
	switch format {
	case fmtYAML:
		out, err := yaml.Marshal(schema)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		out, err := json.MarshalIndent(schema, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	}
}

func init() {
	schemaCmd.Flags().String("command", "", "Command path to generate schema for (e.g. \"jwt parse\")")
	rootCmd.AddCommand(schemaCmd)
}
