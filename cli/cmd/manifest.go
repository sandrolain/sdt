package cmd

import (
	"encoding/json"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// FlagManifest describes a single CLI flag.
type FlagManifest struct {
	Name      string `json:"name"                yaml:"name"`
	Shorthand string `json:"shorthand,omitempty" yaml:"shorthand,omitempty"`
	Usage     string `json:"usage"               yaml:"usage"`
	Default   string `json:"default,omitempty"   yaml:"default,omitempty"`
	Type      string `json:"type"                yaml:"type"`
}

// CommandManifest describes a single CLI command and its subtree.
type CommandManifest struct {
	Name        string            `json:"name"                  yaml:"name"`
	Aliases     []string          `json:"aliases,omitempty"     yaml:"aliases,omitempty"`
	Short       string            `json:"short"                 yaml:"short"`
	Long        string            `json:"long,omitempty"        yaml:"long,omitempty"`
	Deprecated  string            `json:"deprecated,omitempty"  yaml:"deprecated,omitempty"`
	Flags       []FlagManifest    `json:"flags,omitempty"       yaml:"flags,omitempty"`
	Subcommands []CommandManifest `json:"subcommands,omitempty" yaml:"subcommands,omitempty"`
}

// buildCommandManifest recursively builds a CommandManifest from a cobra.Command.
func buildCommandManifest(c *cobra.Command) CommandManifest {
	m := CommandManifest{
		Name:       c.Name(),
		Aliases:    c.Aliases,
		Short:      c.Short,
		Deprecated: c.Deprecated,
	}
	if c.Long != c.Short {
		m.Long = c.Long
	}

	var flags []FlagManifest
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		fm := FlagManifest{
			Name:  f.Name,
			Usage: f.Usage,
			Type:  f.Value.Type(),
		}
		if f.Shorthand != "" {
			fm.Shorthand = f.Shorthand
		}
		if f.DefValue != "" && f.DefValue != "[]" {
			fm.Default = f.DefValue
		}
		flags = append(flags, fm)
	})
	m.Flags = flags

	for _, sub := range c.Commands() {
		if sub.Hidden {
			continue
		}
		m.Subcommands = append(m.Subcommands, buildCommandManifest(sub))
	}
	return m
}

var manifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Emit a machine-readable manifest of all available commands",
	Long: `Emit a JSON or YAML manifest of the full command tree.

Useful for AI agents to auto-discover capabilities without parsing help text.`,
	Run: func(cmd *cobra.Command, args []string) {
		root := cmd.Root()
		type manifestRoot struct {
			Commands []CommandManifest `json:"commands" yaml:"commands"`
		}
		var cmds []CommandManifest
		for _, sub := range root.Commands() {
			if sub.Hidden {
				continue
			}
			cmds = append(cmds, buildCommandManifest(sub))
		}
		result := manifestRoot{Commands: cmds}

		format := getFormat(cmd)
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

func init() {
	rootCmd.AddCommand(manifestCmd)
}
