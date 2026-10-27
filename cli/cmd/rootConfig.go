//go:build !wasm

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ProjectConfig holds the project identity, the search scope and the delivery
// gate declared in .sdt.yaml.
type ProjectConfig struct {
	Project string `json:"project" yaml:"project"`
	Group   string `json:"group"   yaml:"group"`
	// SkipDirs is the project's declared content-verb scope: glob patterns,
	// relative to the project root, pruned from the working tree. Declaring it
	// replaces the shipped defaults rather than extending them; leaving it unset
	// uses them, which is why the defaults ship as comments below and not as a
	// value here.
	SkipDirs []string `json:"skipDirs,omitempty" yaml:"skipDirs,omitempty"`
	// Gate is the project's own delivery ladder: the commands `sdt agent gate`
	// runs, in order, stopping at the first failure. SDT never assumes a
	// toolchain; a project with no `gate:` has no gate, and the command says so.
	Gate []GateStep `json:"gate,omitempty" yaml:"gate,omitempty"`
	// Instructions is the generated set's own configuration: which optional
	// profiles to write and which frontmatter families the lint checks.
	Instructions InstructionsConfig `json:"instructions,omitempty" yaml:"instructions,omitempty"`
	// Research is the declared search-provider order: priority first. Each entry names a
	// provider and may override its endpoint and carry one credential reference. An absent
	// block means the default order (tavily, firecrawl, exa, searxng), where a paid
	// provider is used only when its key is set.
	Research []ResearchProviderConfig `json:"research,omitempty" yaml:"research,omitempty"`
}

// ResearchProviderConfig is one `research:` entry.
type ResearchProviderConfig struct {
	Provider string `json:"provider" yaml:"provider"`
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Key      string `json:"key,omitempty" yaml:"key,omitempty"`
	KeyEnv   string `json:"key_env,omitempty" yaml:"key_env,omitempty"`
	KeyFile  string `json:"key_file,omitempty" yaml:"key_file,omitempty"`
}

// InstructionsConfig is the `instructions:` block. An absent key is the default
// selection (`core` only) with no optional frontmatter family enabled.
type InstructionsConfig struct {
	// Profiles is the resolved generation selection, in register order.
	Profiles []string `json:"profiles,omitempty" yaml:"profiles,omitempty"`
	// Frontmatter names the optional frontmatter families the lint checks.
	Frontmatter []string `json:"frontmatter,omitempty" yaml:"frontmatter,omitempty"`
}

// GateStep is one rung of the project-declared delivery gate. Run is a shell
// command line, executed with /bin/sh -c inside the project root.
type GateStep struct {
	Name string `json:"name" yaml:"name"`
	Run  string `json:"run"  yaml:"run"`
}

// skipDirsComment is the create-only note written under a new .sdt.yaml. It
// documents the key, its shape and the shipped defaults **without** taking a
// value: a written default would stop being a default, freezing every
// bootstrapped project to whatever shipped on the day it was created.
const skipDirsComment = `# skipDirs — globs pruned from the working tree, relative to this file.
# Declaring a list REPLACES the defaults below, it does not extend them.
# Unset means: .git, .hg, .svn, node_modules, .venv
# skipDirs:
#   - dist
#   - .next`

// configOwnedKeys are the top-level keys buildProjectConfigContent renders
// itself. A rewrite replaces them and carries every other top-level block of the
// previous file over verbatim, so `agent init --force` cannot erase a key SDT
// does not know about (`agent:`, `instructions:` and whatever the project adds).
var configOwnedKeys = map[string]bool{
	"project":      true,
	"group":        true,
	"skipDirs":     true,
	"gate":         true,
	"instructions": true,
}

// buildProjectConfigContent renders the YAML body of a .sdt.yaml file. A
// declared skipDirs is rendered as real values; otherwise the create-only
// comment documents the mechanism and leaves the key absent, so the product stays
// free to change its defaults. previous is the file being replaced ("" for a new
// one): its blocks under unowned keys are appended unchanged.
func buildProjectConfigContent(cfg *ProjectConfig, previous string) string {
	lines := []string{fmt.Sprintf("project: %s", cfg.Project)}
	if cfg.Group != "" {
		lines = append(lines, fmt.Sprintf("group: %s", cfg.Group))
	}
	if len(cfg.SkipDirs) > 0 {
		lines = append(lines, "skipDirs:")
		for _, d := range cfg.SkipDirs {
			lines = append(lines, fmt.Sprintf("  - %s", d))
		}
	} else {
		lines = append(lines, skipDirsComment)
	}
	lines = append(lines, gateConfigLines(cfg.Gate)...)
	lines = append(lines, instructionsConfigLines(cfg.Instructions)...)
	content := strings.Join(lines, "\n") + "\n"
	if foreign := foreignConfigBlocks(previous); foreign != "" {
		content += "\n" + foreign
	}
	return content
}

// gateConfigLines renders the declared delivery gate. No declaration renders
// nothing: an absent key means no gate, which is different from a gate that runs
// no steps.
func gateConfigLines(steps []GateStep) []string {
	if len(steps) == 0 {
		return nil
	}
	out := make([]string, 0, 1+2*len(steps))
	out = append(out, "gate:")
	for _, s := range steps {
		out = append(out, "  - name: "+yamlSingleQuoted(s.Name), "    run: "+yamlSingleQuoted(s.Run))
	}
	return out
}

// instructionsConfigLines renders the `instructions:` block. The default
// selection (core alone) renders nothing: absence means core, exactly as an
// absent skipDirs means the shipped defaults, so a project that opted into
// nothing keeps a minimal config.
func instructionsConfigLines(cfg InstructionsConfig) []string {
	var out []string
	if profiles := normalizeProfileList(cfg.Profiles); len(profiles) > 1 {
		out = append(out, "instructions:", "  profiles: ["+strings.Join(profiles, ", ")+"]")
	}
	if len(cfg.Frontmatter) > 0 {
		if len(out) == 0 {
			out = append(out, "instructions:")
		}
		out = append(out, "  frontmatter: ["+strings.Join(cfg.Frontmatter, ", ")+"]")
	}
	return out
}

// normalizeProfileList returns the selection in register order with core
// included, tolerating a hand-edited config.
func normalizeProfileList(profiles []string) []string {
	resolved, err := resolveProfiles(profiles)
	if err != nil {
		return []string{agentProfileCore}
	}
	return resolved
}

// yamlSingleQuoted renders a string as a YAML single-quoted scalar: no escape
// sequences, and the only character needing a rule (the quote itself) doubles.
func yamlSingleQuoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// foreignConfigBlocks returns the top-level YAML blocks of previous whose key is
// not rendered by buildProjectConfigContent. A block starts at a column-zero
// `key:` line and carries its comment lines, its indented body and its blank
// lines: the text is carried verbatim, so a nested comment survives too. Leading
// comments before the first key are always kept.
func foreignConfigBlocks(previous string) string {
	if strings.TrimSpace(previous) == "" {
		return ""
	}
	var preamble, kept, current, carry []string
	seenKey := false
	flush := func() {
		if len(current) == 0 {
			return
		}
		// Trailing blank and column-zero comment lines document the *next*
		// key, not the block they follow: carry them forward.
		body := current
		i := len(body)
		for i > 0 {
			line := body[i-1]
			if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
				i--
				continue
			}
			break
		}
		carry = append([]string{}, body[i:]...)
		if body = trimTrailingBlank(body[:i]); len(body) > 0 {
			if key, ok := blockKey(body); ok && !configOwnedKeys[key] {
				kept = append(kept, strings.TrimLeft(strings.Join(body, "\n"), "\n"))
			}
		}
		current = nil
	}
	for _, line := range strings.Split(strings.TrimRight(previous, "\n"), "\n") {
		if isTopLevelKeyLine(line) {
			flush()
			current = append(append([]string{}, carry...), line)
			carry = nil
			seenKey = true
			continue
		}
		if !seenKey {
			preamble = append(preamble, line)
			continue
		}
		current = append(current, line)
	}
	flush()
	parts := make([]string, 0, 1+len(kept))
	if p := strings.TrimSpace(strings.Join(preamble, "\n")); p != "" {
		parts = append(parts, p)
	}
	parts = append(parts, kept...)
	return strings.Join(parts, "\n\n")
}

// blockKey reports the top-level key a block declares: the first column-zero
// `key:` line, past the comment lines that document it.
func blockKey(lines []string) (string, bool) {
	for _, line := range lines {
		if isTopLevelKeyLine(line) {
			return line[:strings.IndexByte(line, ':')], true
		}
	}
	return "", false
}

// isTopLevelKeyLine reports whether line declares a YAML key at column zero.
func isTopLevelKeyLine(line string) bool {
	if line == "" || line == "-" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
		return false
	}
	i := strings.IndexByte(line, ':')
	if i <= 0 {
		return false
	}
	return !strings.ContainsRune(line[:i], ' ') && !strings.ContainsRune(line[:i], '\t')
}

// trimTrailingBlank drops trailing blank lines from a block.
func trimTrailingBlank(lines []string) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[:end]
}

// findProjectConfig walks up from the current working directory looking for
// a .sdt.yaml file. Returns nil, nil when no file is found.
func findProjectConfig() (*ProjectConfig, error) {
	cfg, _, err := findProjectConfigAt()
	return cfg, err
}

// findProjectConfigAt walks up like findProjectConfig and also reports the
// directory the file was found in. The directory is what a project-root-relative
// pattern resolves against, so a caller that needs one cannot use the config
// alone. Returns nil, "", nil when no file is found; a malformed file degrades to
// nil rather than failing, as every other config read in this codebase does.
func findProjectConfigAt() (*ProjectConfig, string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	for {
		path := filepath.Join(dir, ".sdt.yaml")
		data, err := os.ReadFile(path) //#nosec G304 -- user-controlled project root
		if err == nil {
			var cfg ProjectConfig
			if uerr := yaml.Unmarshal(data, &cfg); uerr == nil {
				return &cfg, dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, "", nil
}

// getProjectAndGroup resolves project and group from (in priority order):
//  1. --project / --group explicit flags
//  2. .sdt.yaml found walking up from CWD
//
// Returns empty strings when neither source provides a value.
func getProjectAndGroup(cmd *cobra.Command) (project, group string) {
	if f := cmd.Flags().Lookup("project"); f != nil && f.Changed {
		if v, err := cmd.Flags().GetString("project"); err == nil {
			project = v
		}
	}
	if f := cmd.Flags().Lookup("group"); f != nil && f.Changed {
		if v, err := cmd.Flags().GetString("group"); err == nil {
			group = v
		}
	}
	if project == "" || group == "" {
		cfg, err := findProjectConfig()
		if err != nil {
			cfg = nil
		}
		if cfg != nil {
			if project == "" {
				project = cfg.Project
			}
			if group == "" {
				group = cfg.Group
			}
		}
	}
	return
}

func loadFileConfig() {
	viper.SetConfigName(sdtConfigFile)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			exitWithError(nil, err)
		}
	}
}

func getInputString(cmd *cobra.Command, args []string) string {
	flags := cmd.Flags()

	if flags.Lookup("file").Changed {
		file := getStringFlag(cmd, "file", false)

		exist, err := fileExists(file)
		exitWithError(cmd, err)
		if !exist {
			exitWithError(cmd, fmt.Errorf(`file "%s" not exist`, file))
		}

		//#nosec G304 -- implementation of generic utility
		res, err := os.ReadFile(file)
		exitWithError(cmd, err)
		return string(res)
	}

	if flags.Lookup("input").Changed {
		return getStringFlag(cmd, "input", true)
	}

	if flags.Lookup("inb64").Changed {
		return string(getBytesBase64Flag(cmd, "inb64", true))
	}

	if len(args) > 0 {
		return strings.Join(args[:], "")
	}
	res, err := io.ReadAll(cmd.InOrStdin())
	exitWithError(cmd, err)
	return string(res)
}

func getInputBytes(cmd *cobra.Command, args []string) []byte {
	flags := cmd.Flags()

	if flags.Lookup("file").Changed {
		file := getStringFlag(cmd, "file", false)

		exist, err := fileExists(file)
		exitWithError(cmd, err)
		if !exist {
			exitWithError(cmd, fmt.Errorf(`file "%s" not exist`, file))
		}
		//#nosec G304 -- implementation of generic utility
		res, err := os.ReadFile(file)
		exitWithError(cmd, err)
		return res
	}

	if flags.Lookup("input").Changed {
		return []byte(getStringFlag(cmd, "input", true))
	}

	if flags.Lookup("inb64").Changed {
		return getBytesBase64Flag(cmd, "inb64", true)
	}

	if len(args) > 0 {
		return []byte(strings.Join(args[:], ""))
	}

	res, err := io.ReadAll(cmd.InOrStdin())
	exitWithError(cmd, err)
	return res
}

func ExecuteByArgs(args []string, in []byte) ([]byte, error) {
	inr := bytes.NewReader(in)
	rootCmd.SetIn(inr)

	origOut := rootCmd.OutOrStdout()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()
	rootCmd.SetIn(nil)
	rootCmd.SetOut(origOut)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
