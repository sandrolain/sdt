package cmd

import (
	"encoding/json"
	"errors"
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
				case doctorStatusFail:
					mark = "FAIL"
					fails++
				case doctorStatusWarn:
					mark = doctorStatusWarn
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
		add("agents.md", doctorStatusFail, "AGENTS.md not found", "run `sdt agent init`")
	case err != nil:
		add("agents.md", doctorStatusFail, err.Error(), "check file permissions")
	case !hasSection(string(agents), agentSectionNameInstructions):
		add("agents.md", doctorStatusFail, "missing instructions block", "run `sdt agent init`")
	default:
		add("agents.md", doctorStatusOK, "present with instructions block", "")
	}

	// Generated instruction files: missing and obsolete.
	missing, obsolete := 0, 0
	for _, f := range instructionFiles("", "") {
		if _, err := os.Stat(filepath.Join(sdtInstrDir, f.name)); os.IsNotExist(err) {
			missing++
		}
	}
	for _, name := range obsoleteInstructionFiles() {
		if _, err := os.Stat(filepath.Join(sdtInstrDir, name)); err == nil {
			obsolete++
		}
	}
	if missing > 0 {
		add("instructions", doctorStatusFail, fmt.Sprintf("%d missing generated file(s)", missing), "run `sdt agent init --force`")
	} else {
		add("instructions", doctorStatusOK, "all generated files present", "")
	}
	if obsolete > 0 {
		add("obsolete", doctorStatusWarn, fmt.Sprintf("%d obsolete file(s)", obsolete), "run `sdt agent init --force` to move them to context/deprecated/")
	} else {
		add("obsolete", doctorStatusOK, "no obsolete files", "")
	}

	// Role profiles: surface role health from the deterministic checks
	// (read-only; never fails the shell).
	checks = append(checks, roleDoctorCheck())

	// Working directories.
	missingDirs := []string{}
	for _, d := range agentVerifyWorkDirs {
		if fi, err := os.Stat(d); os.IsNotExist(err) || (err == nil && !fi.IsDir()) {
			missingDirs = append(missingDirs, d)
		}
	}
	if len(missingDirs) > 0 {
		add("work-dirs", doctorStatusWarn, fmt.Sprintf("missing: %s", strings.Join(missingDirs, ", ")), "run `sdt agent init`")
	} else {
		add("work-dirs", doctorStatusOK, "all present", "")
	}

	// .sdt.yaml project identity.
	if cfg, err := findProjectConfig(); err != nil || cfg == nil {
		add("project-config", doctorStatusFail, ".sdt.yaml not found", "run `sdt agent init --project <p> --group <g> --yes`")
	} else if cfg.Project == "" {
		add("project-config", doctorStatusWarn, ".sdt.yaml has no project id", "set `project:` in .sdt.yaml")
	} else {
		add("project-config", doctorStatusOK, "project "+cfg.Project, "")
	}

	// Derived search cache: report-only staleness (missing is fine).
	if _, err := os.Stat(filepath.Join(mdindex.ManifestFile)); os.IsNotExist(err) {
		add("search-cache", doctorStatusOK, "no cache yet (built on first search)", "")
	} else if err != nil {
		add("search-cache", doctorStatusWarn, err.Error(), "delete .sdt/cache to rebuild")
	} else {
		add("search-cache", doctorStatusOK, "manifest present", "")
	}

	// Corpus exclusions parity is enforced in code; surface the corpus size.
	root, err := os.Getwd()
	if err != nil {
		add("corpus", doctorStatusWarn, err.Error(), "check the working directory")
		return checks
	}
	if res, err := mdindex.Scan(root, nil); err == nil {
		add("corpus", doctorStatusOK, fmt.Sprintf("%d document(s) indexed", len(res.Manifest.Entries)), "")
	} else {
		add("corpus", doctorStatusWarn, err.Error(), "check the context/ tree")
	}

	// Context baseline: report-only size/token attribution of the instruction
	// surface (never a gate).
	checks = append(checks, contextBaselineCheck())

	return checks
}

// roleDoctorCheck surfaces role-profile health as a read-only doctor check:
// presence, drift and owned-path overlap mirror `agent roles check` findings
// without ever failing the shell.
func roleDoctorCheck() doctorCheck {
	findings := roleCheckFindings()
	critical, warns := 0, 0
	for _, f := range findings {
		switch f.Priority {
		case ctxLintCritical:
			critical++
		case ctxLintWarning:
			warns++
		}
	}
	switch {
	case critical > 0:
		return doctorCheck{Name: doctorNameRoles, Status: doctorStatusFail, Detail: fmt.Sprintf("%d role check finding(s)", critical), Hint: "run `sdt agent roles check` and fix the profile set/drift"}
	case warns > 0:
		return doctorCheck{Name: doctorNameRoles, Status: doctorStatusWarn, Detail: fmt.Sprintf("%d role check warning(s)", warns), Hint: "run `sdt agent roles check` and review the flagged profiles"}
	default:
		return doctorCheck{Name: doctorNameRoles, Status: doctorStatusOK, Detail: "profile set, drift and owned paths clean"}
	}
}

// contextBaselineSurface is one measured slice of the always-on/on-demand
// instruction surface: a single file or a directory of markdown files.
type contextBaselineSurface struct {
	name  string
	path  string
	isDir bool
}

// measureContextBaseline returns the file count, bytes, lines and estimated
// tokens for a file or a directory of markdown files. ok is false when the path
// is missing.
func measureContextBaseline(path string, isDir bool) (files, bytes, lines, tokens int, ok bool) {
	measure := func(b []byte) {
		bytes += len(b)
		lines += strings.Count(string(b), "\n")
		tokens += CountTokens(string(b), resolveModelFamily(defaultModel))
	}
	if isDir {
		entries, err := os.ReadDir(path)
		if err != nil {
			return 0, 0, 0, 0, false
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(path, e.Name())) //#nosec G304 -- fixed repo-relative path
			if err != nil {
				continue
			}
			files++
			measure(b)
		}
		return files, bytes, lines, tokens, true
	}
	b, err := os.ReadFile(path) //#nosec G304 -- fixed repo-relative path
	if err != nil {
		return 0, 0, 0, 0, false
	}
	measure(b)
	return 1, bytes, lines, tokens, true
}

// contextBaselineCheck reports the size and token cost of the instruction
// surface (AGENTS.md, the generated instructions, roles and command stubs) as an
// advisory, report-only check: it never fails the shell and never gates.
func contextBaselineCheck() doctorCheck {
	surfaces := []contextBaselineSurface{
		{"agents.md", agentTargetDefault, false},
		{"instructions", sdtInstrDir, true},
		{"roles", sdtRolesDir, true},
		{"commands", sdtCommandsDir, true},
	}
	parts := make([]string, 0, len(surfaces))
	missing := false
	for _, s := range surfaces {
		files, bytes, lines, tokens, ok := measureContextBaseline(s.path, s.isDir)
		if !ok {
			missing = true
			parts = append(parts, s.name+" missing")
			continue
		}
		if s.isDir {
			parts = append(parts, fmt.Sprintf("%s %d files %d lines %dB ~%d tok", s.name, files, lines, bytes, tokens))
		} else {
			parts = append(parts, fmt.Sprintf("%s %d lines %dB ~%d tok", s.name, lines, bytes, tokens))
		}
	}
	detail := strings.Join(parts, " · ")
	if missing {
		return doctorCheck{Name: "context-baseline", Status: doctorStatusWarn, Detail: detail, Hint: "run `sdt agent init` to restore the missing surface"}
	}
	return doctorCheck{Name: "context-baseline", Status: doctorStatusOK, Detail: detail}
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

// gateStepLint is the lint-step name and gateStepTest the test-step name,
// reused by the doctor hint, the gate and the roles project-layer commands.
const gateStepLint = "lint"
const gateStepTest = "test"

// doctor status + check-name vocabulary shared by agentDoctorChecks and
// roleDoctorCheck (goconst: keep the literals centralized).
const (
	doctorStatusOK   = "ok"
	doctorStatusWarn = "warn"
	doctorStatusFail = "fail"
	doctorNameRoles  = "roles"
)

// gateGoListExpr resolves the project Go packages, excluding context/ refs
// (the reference clones hold C/broken-Go fixtures), exactly as Taskfile does.
//
// The `|| echo <sentinel>` keeps the ladder fail-closed: when `go list` fails
// outright (a corpus clone can make the module loader error before the `-e`
// tolerance applies) or resolves nothing, the expansion is the sentinel and the
// go command fails on an unknown package. Without it the substitution was empty,
// the step built or tested nothing and still exited 0, and the gate recorded a
// `pass` for work it never did — the failure laundered into a pass that c20
// forbids.
const gateGoListExpr = "$(go list -e ./... | grep -v /context/ || echo " + gateNoPackagesSentinel + ")"

// gateNoPackagesSentinel is the argument the go steps receive when the package
// list could not be resolved. It is not a valid import path, so the step fails.
const gateNoPackagesSentinel = "__sdt_no_packages__"

// deliveryGateSteps is the fixed, fail-closed verification ladder. It stops at
// the first failure: Build -> Vet -> Lint -> Test.
var deliveryGateSteps = []gateStep{
	{"build", "go", []string{"go build", gateGoListExpr}, true},
	{"vet", "go", []string{"go vet", gateGoListExpr}, true},
	{gateStepLint, "golangci-lint", []string{"run", "./cli/...", "./viewer/...", "./internal/...", "."}, false},
	{gateStepTest, "go", []string{"go test", gateGoListExpr}, true},
}

var agentGateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Run the delivery verification ladder (Build->Vet->Lint->Test)",
	Long: `Run the deterministic delivery gate: build, vet, lint and test, in a fixed
order, stopping at the first failure (fail-closed). This is the strict,
explicit counterpart of the advisory verify-step.

The commands are the project's own Go toolchain (go build/vet/test) plus
golangci-lint; a missing external tool is reported, never silently skipped.

With --record the run is appended to the task file's ` + "`## Review`" + ` block as a
` + "`### Gate`" + ` subsection, one line per step with its real status and duration.
The record is written whether the run passed or failed, and a failing run is
never summarised as a pass. Without --record the gate writes nothing (the CI
path is unchanged).

Examples:
  sdt agent gate
  sdt agent gate --record --plan 20261005-example.md
  sdt agent gate --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		type gateResult struct {
			Step   string `json:"step" yaml:"step"`
			Status string `json:"status" yaml:"status"`
			Detail string `json:"detail,omitempty" yaml:"detail,omitempty"`
		}
		var results []gateResult
		var recorded []gateStepResult
		failed := ""
		for _, step := range deliveryGateSteps {
			exe, err := exec.LookPath(step.Bin)
			if err != nil {
				results = append(results, gateResult{Step: step.Name, Status: "error", Detail: step.Bin + " not found"})
				recorded = append(recorded, gateStepResult{Step: step.Name, Status: "error"})
				failed = step.Name
				break
			}
			started := contextNow()
			out, err := runGateStep(exe, step)
			elapsed := contextNow().Sub(started)
			if err != nil {
				detail := strings.TrimSpace(string(out))
				if len(detail) > 400 {
					detail = detail[:400] + "…"
				}
				results = append(results, gateResult{Step: step.Name, Status: doctorStatusFail, Detail: detail})
				recorded = append(recorded, gateStepResult{Step: step.Name, Status: doctorStatusFail, Duration: elapsed})
				failed = step.Name
				break
			}
			results = append(results, gateResult{Step: step.Name, Status: "pass"})
			recorded = append(recorded, gateStepResult{Step: step.Name, Status: "pass", Duration: elapsed})
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
		if getBoolFlag(cmd, "record", false) {
			// The record is written before the failure exit below: a failed
			// run is exactly what the next session must be able to read back.
			path, err := gateRecordPath(cmd)
			if err == nil {
				err = writeGateRecord(path, contextNow(), failed, recorded)
			}
			if err != nil {
				exitWithError(cmd, err)
			} else {
				outputString(cmd, "recorded gate run in "+path+"\n")
			}
		}
		if failed != "" {
			exitWithError(cmd, fmt.Errorf("delivery gate failed at step %q", failed))
		}
	},
}

// gateRecordPath resolves the task file `--record` writes to. An explicit
// `--plan` is required: the gate never guesses which task file to attribute a
// run to, and never creates the `## Review` block by default.
func gateRecordPath(cmd *cobra.Command) (string, error) {
	plan := getStringFlag(cmd, "plan", false)
	if plan == "" {
		return "", errors.New("--record requires --plan <ref>: name the plan whose task file receives the run")
	}
	return taskFileForRef(sanitizeSlug(getStringFlag(cmd, "phase", false)), "", plan), nil
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

	agentGateCmd.Flags().Bool("record", false, "Append the run to the task file's `## Review` block (requires --plan; without it the gate writes nothing)")
	agentGateCmd.Flags().String("plan", "", "Plan reference whose task file receives the record (required by --record)")
	agentGateCmd.Flags().String("phase", "", "Plan phase whose task file receives the record (default: the plan's whole-plan file)")
}
