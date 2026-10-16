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
