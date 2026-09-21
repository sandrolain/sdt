package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/mdindex"
)

// agent doctor: a report-only health check for the SDT-managed workspace. It
// never mutates anything and never fails the shell: every finding is a report
// line with a severity and a remediation hint.

// doctorCheck is one diagnostic check.
type doctorCheck struct {
	Name   string `json:"name" yaml:"name"`
	Status string `json:"status" yaml:"status"` // ok | warn | fail
	Detail string `json:"detail,omitempty" yaml:"detail,omitempty"`
	Hint   string `json:"hint,omitempty" yaml:"hint,omitempty"`
}

var agentDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Report the health of the SDT workspace (read-only)",
	Long: `Run a read-only health check over the SDT-managed workspace: AGENTS.md
and its instruction block, the generated instruction files (missing/obsolete),
the working directories, the .sdt.yaml project identity, the config file, the
derived search cache and the corpus exclusions.

Unlike 'sdt agent verify' (which fails on CRITICAL issues), doctor always exits
0 and prints a per-check report with a remediation hint.

Examples:
  sdt agent doctor
  sdt agent doctor --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		checks := agentDoctorChecks()
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(checks, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(checks)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			fails, warns := 0, 0
			for _, c := range checks {
				mark := "ok  "
				switch c.Status {
				case "fail":
					mark = "FAIL"
					fails++
				case "warn":
					mark = "warn"
					warns++
				}
				line := fmt.Sprintf("[%s] %-22s %s", mark, c.Name, c.Detail)
				if c.Hint != "" {
					line += " — hint: " + c.Hint
				}
				outputString(cmd, line+"\n")
			}
			outputString(cmd, fmt.Sprintf("\n%d ok, %d warn, %d fail\n", len(checks)-fails-warns, warns, fails))
		}
	},
}

// agentDoctorChecks gathers every diagnostic. Checks are ordered and each is
// independent, so a missing prerequisite degrades the others to warnings.
func agentDoctorChecks() []doctorCheck {
	var checks []doctorCheck
	add := func(name, status, detail, hint string) {
		checks = append(checks, doctorCheck{Name: name, Status: status, Detail: detail, Hint: hint})
	}

	// AGENTS.md + instructions block.
	agents, err := os.ReadFile(agentTargetDefault) //#nosec G304 -- fixed repo-path check
	switch {
	case os.IsNotExist(err):
		add("agents.md", "fail", "AGENTS.md not found", "run `sdt agent init`")
	case err != nil:
		add("agents.md", "fail", err.Error(), "check file permissions")
	case !hasSection(string(agents), agentSectionNameInstructions):
		add("agents.md", "fail", "missing instructions block", "run `sdt agent init`")
	default:
		add("agents.md", "ok", "present with instructions block", "")
	}

	// Generated instruction files: missing and obsolete.
	missing, obsolete := 0, 0
	for _, f := range instructionFiles("", "") {
		if _, err := os.Stat(filepath.Join(sdtInstrDir, f.name)); os.IsNotExist(err) {
			missing++
		}
	}
	for _, name := range obsoleteInstructionFiles {
		if _, err := os.Stat(filepath.Join(sdtInstrDir, name)); err == nil {
			obsolete++
		}
	}
	if missing > 0 {
		add("instructions", "fail", fmt.Sprintf("%d missing generated file(s)", missing), "run `sdt agent init --force`")
	} else {
		add("instructions", "ok", "all generated files present", "")
	}
	if obsolete > 0 {
		add("obsolete", "warn", fmt.Sprintf("%d obsolete file(s)", obsolete), "run `sdt agent init --force` to archive them")
	} else {
		add("obsolete", "ok", "no obsolete files", "")
	}

	// Working directories.
	missingDirs := []string{}
	for _, d := range agentVerifyWorkDirs {
		if fi, err := os.Stat(d); os.IsNotExist(err) || (err == nil && !fi.IsDir()) {
			missingDirs = append(missingDirs, d)
		}
	}
	if len(missingDirs) > 0 {
		add("work-dirs", "warn", fmt.Sprintf("missing: %s", strings.Join(missingDirs, ", ")), "run `sdt agent init`")
	} else {
		add("work-dirs", "ok", "all present", "")
	}

	// .sdt.yaml project identity.
	if cfg, err := findProjectConfig(); err != nil || cfg == nil {
		add("project-config", "fail", ".sdt.yaml not found", "run `sdt agent init --project <p> --group <g> --yes`")
	} else if cfg.Project == "" {
		add("project-config", "warn", ".sdt.yaml has no project id", "set `project:` in .sdt.yaml")
	} else {
		add("project-config", "ok", "project "+cfg.Project, "")
	}

	// Derived search cache: report-only staleness (missing is fine).
	if _, err := os.Stat(filepath.Join(mdindex.ManifestFile)); os.IsNotExist(err) {
		add("search-cache", "ok", "no cache yet (built on first search)", "")
	} else if err != nil {
		add("search-cache", "warn", err.Error(), "delete .sdt/cache to rebuild")
	} else {
		add("search-cache", "ok", "manifest present", "")
	}

	// Corpus exclusions parity is enforced in code; surface the corpus size.
	root, err := os.Getwd()
	if err != nil {
		add("corpus", "warn", err.Error(), "check the working directory")
		return checks
	}
	if res, err := mdindex.Scan(root, nil); err == nil {
		add("corpus", "ok", fmt.Sprintf("%d document(s) indexed", len(res.Manifest.Entries)), "")
	} else {
		add("corpus", "warn", err.Error(), "check the context/ tree")
	}

	return checks
}

// ── delivery gate ──────────────────────────────────────────────────────────────

// gateStep is one rung of the delivery verification ladder. UseShell marks the
// go steps whose package list is resolved at runtime by go list, excluding the
// context/ tree (which holds reference clones with C/broken-Go fixtures, exactly
// as the project's Taskfile does).
type gateStep struct {
	Name     string
	Bin      string
	Args     []string
	UseShell bool
}

// gateStepLint is the lint-step name, reused by the doctor hint and the gate.
const gateStepLint = "lint"

// gateGoListExpr resolves the project Go packages, excluding context/ refs
// (the reference clones hold C/broken-Go fixtures), exactly as Taskfile does.
const gateGoListExpr = "$(go list -e ./... | grep -v /context/)"

// deliveryGateSteps is the fixed, fail-closed verification ladder. It stops at
// the first failure: Build -> Vet -> Lint -> Test.
var deliveryGateSteps = []gateStep{
	{"build", "go", []string{"go build", gateGoListExpr}, true},
	{"vet", "go", []string{"go vet", gateGoListExpr}, true},
	{gateStepLint, "golangci-lint", []string{"run", "./cli/...", "./viewer/...", "./internal/...", "."}, false},
	{"test", "go", []string{"go test", gateGoListExpr}, true},
}

var agentGateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Run the delivery verification ladder (Build->Vet->Lint->Test)",
	Long: `Run the deterministic delivery gate: build, vet, lint and test, in a fixed
order, stopping at the first failure (fail-closed). This is the strict,
explicit counterpart of the advisory verify-step.

The commands are the project's own Go toolchain (go build/vet/test) plus
golangci-lint; a missing external tool is reported, never silently skipped.

Examples:
  sdt agent gate
  sdt agent gate --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		type gateResult struct {
			Step   string `json:"step" yaml:"step"`
			Status string `json:"status" yaml:"status"`
			Detail string `json:"detail,omitempty" yaml:"detail,omitempty"`
		}
		var results []gateResult
		failed := ""
		for _, step := range deliveryGateSteps {
			exe, err := exec.LookPath(step.Bin)
			if err != nil {
				results = append(results, gateResult{Step: step.Name, Status: "error", Detail: step.Bin + " not found"})
				failed = step.Name
				break
			}
			out, err := runGateStep(exe, step)
			if err != nil {
				detail := strings.TrimSpace(string(out))
				if len(detail) > 400 {
					detail = detail[:400] + "…"
				}
				results = append(results, gateResult{Step: step.Name, Status: "fail", Detail: detail})
				failed = step.Name
				break
			}
			results = append(results, gateResult{Step: step.Name, Status: "pass"})
		}
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(results, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(results)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, r := range results {
				outputString(cmd, fmt.Sprintf("[%s] %s %s\n", r.Status, r.Step, r.Detail))
			}
		}
		if failed != "" {
			exitWithError(cmd, fmt.Errorf("delivery gate failed at step %q", failed))
		}
	},
}

// runGateStep runs one ladder step. Shell steps expand the package expression
// (go list | grep), direct steps exec the fixed argument list.
func runGateStep(exe string, step gateStep) ([]byte, error) {
	if step.UseShell {
		//#nosec G204 -- fixed tool + fixed pipeline, no user input
		return exec.Command("/bin/sh", "-c", strings.Join(step.Args, " ")).CombinedOutput()
	}
	//#nosec G204 -- fixed tool + fixed argument list, no user input
	return exec.Command(exe, step.Args...).CombinedOutput()
}

func init() {
	agentCmd.AddCommand(agentDoctorCmd, agentGateCmd)
}
