package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

type parallelResult struct {
	Index    int      `json:"index" yaml:"index"`
	Argv     []string `json:"argv" yaml:"argv"`
	ExitCode int      `json:"exit_code" yaml:"exit_code"`
	Stdout   string   `json:"stdout" yaml:"stdout"`
	Stderr   string   `json:"stderr" yaml:"stderr"`
	Error    string   `json:"error,omitempty" yaml:"error,omitempty"`
}

// parseParallelArgs decodes every --cmd value as a JSON string array. It
// validates all of them before any process is started.
func parseParallelArgs(raw []string) ([][]string, error) {
	if len(raw) == 0 {
		return nil, errors.New("at least one --cmd required, e.g. --cmd '[\"go\",\"version\"]'")
	}
	argvs := make([][]string, 0, len(raw))
	for i, s := range raw {
		var argv []string
		if err := json.Unmarshal([]byte(s), &argv); err != nil {
			return nil, fmt.Errorf("--cmd #%d: expected a JSON string array: %w", i+1, err)
		}
		if len(argv) == 0 || argv[0] == "" {
			return nil, fmt.Errorf("--cmd #%d: argv must start with a non-empty executable", i+1)
		}
		argvs = append(argvs, argv)
	}
	return argvs, nil
}

// runParallel starts every argv directly (no shell) and waits for all of them.
// Results keep submission order; one failing command never stops the others.
func runParallel(argvs [][]string) []parallelResult {
	results := make([]parallelResult, len(argvs))
	var wg sync.WaitGroup
	for i, argv := range argvs {
		wg.Add(1)
		go func(i int, argv []string) {
			defer wg.Done()
			var stdout, stderr bytes.Buffer
			c := exec.Command(argv[0], argv[1:]...) //nolint:gosec // the caller names the argv explicitly; no shell is involved
			c.Stdout = &stdout
			c.Stderr = &stderr
			res := parallelResult{Index: i + 1, Argv: argv}
			err := c.Run()
			var exitErr *exec.ExitError
			switch {
			case err == nil:
				res.ExitCode = 0
			case errors.As(err, &exitErr):
				res.ExitCode = exitErr.ExitCode()
			default:
				res.ExitCode = -1
				res.Error = err.Error()
			}
			res.Stdout = stdout.String()
			res.Stderr = stderr.String()
			results[i] = res
		}(i, argv)
	}
	wg.Wait()
	return results
}

func renderParallelText(results []parallelResult) string {
	var b strings.Builder
	for _, r := range results {
		argv, err := json.Marshal(r.Argv)
		if err != nil {
			argv = []byte("[]")
		}
		fmt.Fprintf(&b, "== [%d] %s  exit %d\n", r.Index, argv, r.ExitCode)
		if r.Error != "" {
			fmt.Fprintf(&b, "error: %s\n", r.Error)
		}
		if r.Stdout != "" {
			b.WriteString("--- stdout\n")
			b.WriteString(r.Stdout)
			if !strings.HasSuffix(r.Stdout, "\n") {
				b.WriteString("\n")
			}
		}
		if r.Stderr != "" {
			b.WriteString("--- stderr\n")
			b.WriteString(r.Stderr)
			if !strings.HasSuffix(r.Stderr, "\n") {
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

var parallelCmd = &cobra.Command{
	Use:   "parallel --cmd '<json argv>' [--cmd '<json argv>' ...]",
	Short: "Run independent commands concurrently and collect their results",
	Long: `Run several independent commands at once and report each one's output and exit status.

Each --cmd is a JSON array of strings: the executable followed by its arguments.
Commands run directly, never through a shell, so no expansion or quoting applies.
All --cmd values are validated before any command starts.

RETURNS — one result per --cmd, in submission order: its argv, exit code,
stdout and stderr. --format json|yaml gives the same fields as structured data.
Exit status is 0 when every command succeeds, 3 when some fail (partial result),
and 1 when all fail.

EXAMPLE — sdt parallel --cmd '["git","status","--short"]' --cmd '["git","log","-1","--oneline"]'`,
	Run: func(cmd *cobra.Command, args []string) {
		raw := getStringArrayFlag(cmd, "cmd", false)
		argvs, err := parseParallelArgs(raw)
		exitWithError(cmd, err)
		if err != nil {
			return
		}

		results := runParallel(argvs)

		failed := 0
		for _, r := range results {
			if r.ExitCode != 0 {
				failed++
			}
		}

		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(results, "", "  ")
			exitWithError(cmd, err)
			outputString(cmd, string(out)+"\n")
		case fmtYAML:
			out, err := yaml.Marshal(results)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			outputString(cmd, renderParallelText(results))
		}

		switch {
		case failed == 0:
		case failed == len(results):
			exitWithError(cmd, fmt.Errorf("all %d commands failed", failed))
		default:
			exitWithJunk(cmd, fmt.Sprintf("%d of %d commands failed", failed, len(results)))
		}
	},
}

func init() {
	parallelCmd.Flags().StringArray("cmd", nil, "JSON argv array to run (repeatable)")
	rootCmd.AddCommand(parallelCmd)
}
