package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// errConfiguredCommand is the refusal of a configured agent command the viewer
// cannot run. It is a typed error rather than a message because two readers need
// it: the log at start, and the sentence the panel shows when a start fails. The
// name avoids the wire-level errAgentCommand, which refuses a malformed request
// body rather than an unusable configuration.
var errConfiguredCommand = errors.New("the configured agent command cannot be run")

// viewerConfig is the subset of .sdt.yaml the viewer consumes; unknown keys are
// ignored.
type viewerConfig struct {
	Project string             `yaml:"project"`
	Group   string             `yaml:"group"`
	Search  viewerSearchConfig `yaml:"search"`
	Agent   viewerAgentConfig  `yaml:"agent"`
}

type viewerSearchConfig struct {
	// Semantic enables the optional /api/search?semantic=1 branch.
	Semantic bool `yaml:"semantic"`
	// SemanticBuild is the on-demand snapshot build switch for the semantic map;
	// nil (absent) means enabled (default true). A pointer distinguishes
	// "not set" from an explicit false.
	SemanticBuild *bool `yaml:"semantic_build"`
	// Model overrides the embedding model id (default BASE8M). Empty disables
	// this override.
	Model string `yaml:"model"`
}

// viewerAgentConfig declares the agent this project runs. Declaring it is what
// enables the bridge: no other key switches one on, and a request never names a
// command — the browser can ask the viewer to start *this* one and nothing else.
type viewerAgentConfig struct {
	// Command is the agent's argv, executed directly. There is no shell, so there
	// is nothing to quote and nothing to interpolate; the first element is either a
	// program name, resolved at spawn time, or an explicit path.
	Command []string `yaml:"command"`
	// Model is the single-model spelling, kept for existing projects. It is the
	// one-entry form of Models and is read only when Models is absent, so a project
	// that declares both gets the list, not a silent merge.
	Model string `yaml:"model"`
	// Models is the list the bridge tries in order at every session open — a fresh
	// one, or the one a restart carries — and keeps the first the agent accepts. It
	// is the id the agent itself offers, taken verbatim: the viewer never maps a
	// name. A blank entry names nothing and is dropped; duplicates collapse.
	Models []string `yaml:"models"`
	// Log is the wire log's declared path, resolved against the project root. Its
	// presence is what turns the capture on: absent or blank means off, so the key
	// is additive and nothing has to be removed to stop logging. The path may
	// carry {timestamp} and {session}, which give each run or each conversation its
	// own file; without them every run appends to the one file.
	Log string `yaml:"log"`
}

// agentCommand returns the declared argv, or nil when the project declares no
// usable agent. An empty argv, or one whose program is blank, counts as not
// declared: it could never run, and reporting it here would put the refusal
// where no reader is looking — the start is where a failure belongs, because
// that is where a reader asked for it.
func (c viewerConfig) agentCommand() []string {
	if len(c.Agent.Command) == 0 || strings.TrimSpace(c.Agent.Command[0]) == "" {
		return nil
	}
	return c.Agent.Command
}

// agentModels returns the declared models in the order the bridge should try
// them, or nil when the project declares none. Blanks count as absent the way a
// blank argv does — they name nothing — and a repeated id is tried once.
func (c viewerConfig) agentModels() []string {
	raw := c.Agent.Models
	if len(raw) == 0 {
		raw = []string{c.Agent.Model}
	}
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, model := range raw {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		out = append(out, model)
	}
	return out
}

// agentLog returns the declared wire-log path, or the empty string when the
// project declares none. A blank one counts as absent the way a blank model does:
// it names no file, and treating whitespace as a path would create a file called
// by nothing.
func (c viewerConfig) agentLog() string {
	return strings.TrimSpace(c.Agent.Log)
}

// loadViewerConfig reads root/.sdt.yaml for viewer-relevant keys. A missing or
// malformed file yields the zero value (no search branch, no agent) without
// error, so a broken config never keeps the viewer from serving the corpus.
func loadViewerConfig(root string) viewerConfig {
	data, err := os.ReadFile(filepath.Join(root, sdtConfigFile)) //#nosec G304 -- user project config
	if err != nil {
		return viewerConfig{}
	}
	var cfg viewerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return viewerConfig{}
	}
	return cfg
}

// resolveCommand turns a configured argv into one that can be executed. A bare
// program name is looked up on PATH and then beside the running binary, exactly
// as the viewer's own sdt companion is; an explicit path is taken as given and
// must be executable. The remaining arguments pass through untouched.
//
// It runs at spawn time rather than at load, so a command that cannot be run
// produces the sentence a reader can act on, at the moment they asked for it,
// instead of a viewer that refuses to start over a line in a file.
func resolveCommand(argv []string, self string) ([]string, error) {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return nil, fmt.Errorf("%w: the command is empty", errConfiguredCommand)
	}
	program := argv[0]
	if strings.ContainsRune(program, filepath.Separator) {
		abs, err := filepath.Abs(program)
		if err != nil || !isExecutableFile(abs) {
			return nil, fmt.Errorf("%w: %q is not an executable file", errConfiguredCommand, program)
		}
		return append([]string{abs}, argv[1:]...), nil
	}
	if found, err := exec.LookPath(program); err == nil {
		return append([]string{found}, argv[1:]...), nil
	}
	if self != "" {
		candidate := filepath.Join(filepath.Dir(self), program)
		if isExecutableFile(candidate) {
			return append([]string{candidate}, argv[1:]...), nil
		}
	}
	return nil, fmt.Errorf("%w: %q is neither on PATH nor beside the viewer", errConfiguredCommand, program)
}
