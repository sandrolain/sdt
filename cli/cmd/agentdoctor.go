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
the working directories, the .sdt.yaml project identity, the declared delivery
gate, the config file, the derived search cache and the corpus exclusions.

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

	// Generated instruction files: missing and obsolete. Only the selected
	// profiles count: an unselected domain is neither missing nor obsolete.
	profiles := currentProfiles()
	add("profiles", doctorStatusOK, agentProfileSummary(profiles), "add one with `sdt agent init --with <profile>` or `sdt agent refresh --with <profile>`")
	missing, obsolete := 0, 0
	for _, f := range instructionFilesFor(profiles, "", "") {
		if _, err := os.Stat(filepath.Join(sdtInstrDir, f.name)); os.IsNotExist(err) {
			missing++
		}
	}
	for _, name := range obsoleteInstructionFiles(profiles) {
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
		add("obsolete", doctorStatusWarn, fmt.Sprintf("%d obsolete file(s)", obsolete), "run `sdt agent init --force` to move them to context/trash/")
	} else {
		add("obsolete", doctorStatusOK, "no obsolete files", "")
	}

	// Role profiles: surface role health from the deterministic checks
	// (read-only; never fails the shell). Roles are opt-in, so a project that
	// did not select the profile has nothing to check.
	checks = append(checks, roleDoctorCheck(profiles))

	// Working directories.
	wd := workDirsDoctorCheck()
	add(wd.Name, wd.Status, wd.Detail, wd.Hint)

	// .sdt.yaml project identity.
	if cfg, err := findProjectConfig(); err != nil || cfg == nil {
		add("project-config", doctorStatusFail, ".sdt.yaml not found", "run `sdt agent init --project <p> --group <g> --yes`")
	} else if cfg.Project == "" {
		add("project-config", doctorStatusWarn, ".sdt.yaml has no project id", "set `project:` in .sdt.yaml")
	} else {
		add("project-config", doctorStatusOK, "project "+cfg.Project, "")
	}

	// Delivery gate: declared by the project, never assumed. An absent
	// declaration is a warning, not a failure: a project may run its checks in
	// CI instead.
	if cfg, err := findProjectConfig(); err != nil || cfg == nil || len(cfg.Gate) == 0 {
		add("delivery-gate", doctorStatusWarn, "no gate declared", "add a `gate:` list to .sdt.yaml so `sdt agent gate` runs your own checks")
	} else {
		add("delivery-gate", doctorStatusOK, fmt.Sprintf("%d declared step(s): %s", len(cfg.Gate), strings.Join(gateStepNames(cfg.Gate), ", ")), "")
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
	checks = append(checks, sessionStartCheck())

	return checks
}

// workDirsDoctorCheck reports the context/ layout the generated set expects.
func workDirsDoctorCheck() doctorCheck {
	var missing []string
	for _, d := range agentVerifyWorkDirs {
		if fi, err := os.Stat(d); os.IsNotExist(err) || (err == nil && !fi.IsDir()) {
			missing = append(missing, d)
		}
	}
	if len(missing) > 0 {
		return doctorCheck{Name: "work-dirs", Status: doctorStatusWarn, Detail: "missing: " + strings.Join(missing, ", "), Hint: "run `sdt agent init`"}
	}
	return doctorCheck{Name: "work-dirs", Status: doctorStatusOK, Detail: "all present"}
}

// roleDoctorCheck surfaces role-profile health as a read-only doctor check:
// presence, drift and owned-path overlap mirror `agent roles check` findings
// without ever failing the shell.
func roleDoctorCheck(profiles []string) doctorCheck {
	if !profileSelected(profiles, "roles") {
		return doctorCheck{Name: doctorNameRoles, Status: doctorStatusOK, Detail: "not selected", Hint: "add the profile with `sdt agent init --with roles`"}
	}
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

// agentSurfaceCheck reports the size and token cost of the agent surface as an
// advisory, report-only check: it never fails the shell and never gates. The two
// halves are named apart — the AGENTS.md block is paid on every turn, while the
// instruction modules, roles and command stubs are read on demand — because a
// single figure that adds them reads as a baseline nobody actually loads.
func contextBaselineCheck() doctorCheck {
	surfaces := []contextBaselineSurface{
		{"agents.md (every turn)", agentTargetDefault, false},
		{"instructions (on demand)", sdtInstrDir, true},
		{"roles (on demand)", sdtRolesDir, true},
		{"commands (on demand)", sdtCommandsDir, true},
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
		return doctorCheck{Name: "agent-surface", Status: doctorStatusWarn, Detail: detail, Hint: "run `sdt agent init` to restore the missing surface"}
	}
	return doctorCheck{Name: "agent-surface", Status: doctorStatusOK, Detail: detail}
}

// sessionStartCheck reports the bounded digest a session begins with. Its
// absence is a warning rather than a failure — a project that has not reindexed
// yet still works — but the size is reported against the cap that makes the read
// predictable.
func sessionStartCheck() doctorCheck {
	data, err := os.ReadFile(sdtEssentialsFile) //#nosec G304 -- fixed corpus path
	if err != nil {
		return doctorCheck{
			Name:   "session-start",
			Status: doctorStatusWarn,
			Detail: sdtEssentialsFile + " missing",
			Hint:   "run `sdt context reindex` (or `sdt agent init`)",
		}
	}
	words := len(strings.Fields(string(data)))
	status, hint := doctorStatusOK, ""
	if words > ctxEssentialsWordBudget {
		status = doctorStatusWarn
		hint = "the digest is over its cap: split it or raise the cap deliberately"
	}
	return doctorCheck{Name: "session-start", Status: status, Detail: fmt.Sprintf("%s %d words (cap %d)", sdtEssentialsFile, words, ctxEssentialsWordBudget), Hint: hint}
}

// ── delivery gate ──────────────────────────────────────────────────────────────

// gateStepLint is the lint-step name and gateStepTest the test-step name,
// reused by the doctor hint and the roles project-layer commands.
const gateStepLint = "lint"
const gateStepTest = "test"

// doctor status + check-name vocabulary shared by agentDoctorChecks and
// roleDoctorCheck (goconst: keep the literals centralized).
const (
	doctorStatusOK   = "ok"
	doctorStatusWarn = "warn"
	doctorStatusFail = "fail"
	doctorStatusPass = "pass"
	doctorNameRoles  = "roles"
)

// agentGateCmd runs the project's own delivery gate: the `gate:` steps declared
// in .sdt.yaml, in order, stopping at the first failure. SDT ships no ladder of
// its own — a Go build, a cargo check and an npm test are all one declaration,
// and a project that declares none has no gate rather than a wrong one.
var agentGateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Run the delivery gate declared in .sdt.yaml",
	Long: `Run the project's own delivery gate: the commands declared under ` + "`gate:`" + ` in
.sdt.yaml, in order, stopping at the first failure (fail-closed). This is the
strict, explicit counterpart of the advisory verify-step.

Each step's ` + "`run`" + ` is a shell command line executed with /bin/sh -c in the
project root, so a step can expand its own package list. Nothing is assumed about
the toolchain: with no ` + "`gate:`" + ` declared the command reports "no gate declared"
and exits 0, and a missing tool surfaces as that step failing.

A failing step's console block is the tail of its output — the head is usually the
echo of the command — and the whole output is written to
context/tmp/gate-<step>-<timestamp>.log, named in the block and in the structured
shape's ` + "`log`" + ` field. The console stays short; the evidence survives.

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
			// Log is the path of the failing step's full output, written under
			// context/tmp: the console keeps the tail, the evidence survives.
			Log string `json:"log,omitempty" yaml:"log,omitempty"`
		}
		cfg, err := findProjectConfig()
		exitWithError(cmd, err)
		if cfg == nil {
			exitWithError(cmd, fmt.Errorf("no %s found walking up from the current directory", sdtConfigFile))
			return
		}
		if err := validateGateSteps(cfg.Gate); err != nil {
			exitWithError(cmd, err)
			return
		}
		var results []gateResult
		var recorded []gateStepResult
		failed := ""
		if len(cfg.Gate) == 0 {
			outputString(cmd, "no gate declared: add a `gate:` list to "+sdtConfigFile+"\n")
			return
		}
		for _, step := range cfg.Gate {
			started := contextNow()
			out, err := runGateStep(step.Run)
			elapsed := contextNow().Sub(started)
			if err != nil {
				detail, logPath := gateFailureDetail(step.Name, out)
				results = append(results, gateResult{Step: step.Name, Status: doctorStatusFail, Detail: detail, Log: logPath})
				recorded = append(recorded, gateStepResult{Step: step.Name, Status: doctorStatusFail, Duration: elapsed})
				failed = step.Name
				break
			}
			results = append(results, gateResult{Step: step.Name, Status: doctorStatusPass})
			recorded = append(recorded, gateStepResult{Step: step.Name, Status: doctorStatusPass, Duration: elapsed})
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
				// A failing step gets its own block: the tail of the output, then
				// the path that carries all of it. The one-line shape would bury
				// the error the reader needs.
				if r.Status == doctorStatusFail && (r.Detail != "" || r.Log != "") {
					outputString(cmd, fmt.Sprintf("[%s] %s\n", r.Status, r.Step))
					for _, line := range strings.Split(r.Detail, "\n") {
						outputString(cmd, "  "+line+"\n")
					}
					if r.Log != "" {
						outputString(cmd, "  full output: "+r.Log+"\n")
					}
					continue
				}
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

// gateRecordPath resolves the document `--record` writes to. An explicit
// `--plan` is required: the gate never guesses where a run belongs. The plan is
// the execution ledger, so the record goes into the plan itself unless the plan
// was split into a task file — then it goes to the task file that executes the
// phase named by `--phase`.
func gateRecordPath(cmd *cobra.Command) (string, error) {
	plan := getStringFlag(cmd, "plan", false)
	if plan == "" {
		return "", errors.New("--record requires --plan <ref>: name the plan the run belongs to")
	}
	if path := existingTaskFileForPlan(sanitizeSlug(getStringFlag(cmd, "phase", false)), plan); path != "" {
		return path, nil
	}
	path, err := resolveContextLintPath(plan)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path) //#nosec G304 -- path resolved within context/
	if err != nil {
		return "", err
	}
	if parseFrontmatterField(string(data), ctxFrontmatterKind) != ctxTypePlan {
		return "", fmt.Errorf("%s is not a plan: --plan names the plan the run belongs to", plan)
	}
	return path, nil
}

// existingTaskFileForPlan returns the task file that executes a plan (or one of
// its phases), or "" when the plan has none — the unsplit case, where the plan
// carries its own checklist.
func existingTaskFileForPlan(phase, plan string) string {
	slug := taskSlugFromPlan(plan)
	if phase != "" {
		if matches, err := taskFilesForPhase(phase, slug); err == nil && len(matches) > 0 {
			return newestTaskFile(matches)
		}
		return ""
	}
	if matches, err := taskPlanFiles(slug); err == nil && len(matches) > 0 {
		return newestTaskFile(matches)
	}
	return ""
}

// validateGateSteps refuses a malformed declaration before anything runs, so a
// typo cannot pass silently as a gate that executed nothing.
func validateGateSteps(steps []GateStep) error {
	for i, s := range steps {
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("gate step %d has no name", i+1)
		}
		if strings.TrimSpace(s.Run) == "" {
			return fmt.Errorf("gate step %q has no run command", s.Name)
		}
	}
	return nil
}

// gateStepNames returns the declared step names, in order.
func gateStepNames(steps []GateStep) []string {
	names := make([]string, 0, len(steps))
	for _, s := range steps {
		names = append(names, s.Name)
	}
	return names
}

// runGateStep runs one declared step through the shell: the command line is the
// project's own, written in its own .sdt.yaml, so it may expand a package list
// or pipe a result.
// gateLogTailLines is how many lines of a failing step's output the console
// shows. The first bytes of a failing suite are the echo of the command, not the
// error, which is why the tail is the hint and the file is the record.
const gateLogTailLines = 20

// gateFailureDetail keeps what a reader needs from a failing step: the tail of
// its output for the console, and the path of the file holding all of it. The
// write is best-effort — a read-only tree degrades to the tail alone, never to a
// lost gate result.
func gateFailureDetail(step string, out []byte) (detail, logPath string) {
	text := strings.TrimRight(string(out), "\n")
	if text == "" {
		return "", ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > gateLogTailLines {
		lines = lines[len(lines)-gateLogTailLines:]
	}
	return strings.Join(lines, "\n"), writeGateLog(step, text)
}

// writeGateLog stores a failing step's full output under the project's
// context/tmp and returns the project-relative path, or "" when it cannot be
// written.
func writeGateLog(step, text string) string {
	_, root, err := findProjectConfigAt()
	if err != nil || root == "" {
		return ""
	}
	dir := filepath.Join(root, "context", "tmp")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return ""
	}
	name := sanitizeSlug(step)
	if name == "" {
		name = "step"
	}
	path := filepath.Join(dir, fmt.Sprintf("gate-%s-%s.log", name, contextNow().UTC().Format("20060102-150405")))
	//#nosec G306 -- a temp log inside the project, readable by its owner only
	if err := os.WriteFile(path, []byte(text+"\n"), 0o600); err != nil {
		return ""
	}
	if rel, rerr := filepath.Rel(root, path); rerr == nil {
		return filepath.ToSlash(rel)
	}
	return path
}

func runGateStep(command string) ([]byte, error) {
	//#nosec G204 -- the project's own declared command, executed from its config
	return exec.Command("/bin/sh", "-c", command).CombinedOutput()
}

func init() {
	agentCmd.AddCommand(agentDoctorCmd, agentGateCmd)

	agentGateCmd.Flags().Bool("record", false, "Append the run to the task file's `## Review` block (requires --plan; without it the gate writes nothing)")
	agentGateCmd.Flags().String("plan", "", "Plan reference whose task file receives the record (required by --record)")
	agentGateCmd.Flags().String("phase", "", "Plan phase whose task file receives the record (default: the plan's whole-plan file)")
}
