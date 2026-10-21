package cmd

import (
	"encoding/json"
	"errors"
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
	Command     string      `json:"command"               yaml:"command"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	Input       *JSONSchema `json:"input"                yaml:"input"`
	Flags       *JSONSchema `json:"flags"                yaml:"flags"`
}

// cobraFlagTypeToJSONType maps pflag type names to JSON Schema types.
func cobraFlagTypeToJSONType(t string) string {
	switch t {
	case typeInt, "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	case typeStringArray, "stringSlice":
		return "array"
	default:
		return typeString
	}
}

// commandFlagSchema converts one pflag into its JSON Schema entry.
func commandFlagSchema(f *pflag.Flag) *JSONSchema {
	s := &JSONSchema{
		Description: f.Usage,
		Type:        cobraFlagTypeToJSONType(f.Value.Type()),
	}
	if f.DefValue != "" && f.DefValue != "[]" && f.DefValue != flagValueFalse {
		s.Default = f.DefValue
	}
	if f.Value.Type() == typeStringArray || f.Value.Type() == "stringSlice" {
		s.Items = &JSONSchema{Type: typeString}
	}
	return s
}

// buildFlagSchema builds a JSON Schema describing a command's flags: its own and
// the globals it inherits.
//
// It reads `Flags()` no longer. That method returns only what the command
// declares on `Flags()`, while flags added with `PersistentFlags()` live in a
// separate set — so for a command like `crawldown`, which declares its whole
// option set persistently, the walk found nothing and the schema emitted a bare
// `{"type": "object"}`, telling an agent the verb takes no options. Measured
// before the fix: 83 of 173 commands.
//
// `LocalFlags()` and `InheritedFlags()` together are the set a caller can
// actually pass. A name declared in both — a command shadowing a global — is
// reported once, from the local declaration, because that is the flag the
// command will parse; `InheritedFlags()` alone would be ambiguous, and iterating
// both sets into one map would let insertion order decide.
func buildFlagSchema(c *cobra.Command) *JSONSchema {
	props := map[string]*JSONSchema{}
	local := map[string]struct{}{}
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		local[f.Name] = struct{}{}
		props[f.Name] = commandFlagSchema(f)
	})
	c.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		if _, shadowed := local[f.Name]; shadowed {
			return
		}
		props[f.Name] = commandFlagSchema(f)
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
			Type:        typeString,
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

Without a command, emits a schema for every command as a JSON array. Name one
command to get only its schema: ` + "`sdt schema jwt parse`" + ` or the equivalent
` + "`--command \"jwt parse\"`" + `. The two forms are alternatives; passing both is refused.

Examples:
  sdt schema                          # all commands
  sdt schema jwt parse                # single command
  sdt schema --command "jwt parse"    # the same, as one flag
  sdt schema --format yaml            # YAML output`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmdPath := getStringFlag(cmd, "command", false)
		if len(args) > 0 {
			if cmdPath != "" {
				exitWithError(cmd, errors.New("name the command positionally or with --command, not both"))
				return
			}
			cmdPath = strings.Join(args, " ")
		}
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

// collectSchemas reports the schema of every leaf command the shared
// enumeration yields, so the schema and the generated docs describe the same
// command set by construction rather than by two walks agreeing.
func collectSchemas(root *cobra.Command, out *[]CommandSchema) {
	var leaves []leafCommand
	for _, sub := range root.Commands() {
		collectCommandTree(sub, "", &leaves)
	}
	for _, l := range leaves {
		*out = append(*out, buildCommandSchema(l.c))
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
