package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// agentVerifyWorkDirs are the context/ directories that a well-formed
// project must have, including the executable-payloads dir added by the
// scripts convention.
var agentVerifyWorkDirs = []string{
	sdtWorkDir,
	sdtPlanDir,
	sdtAnalysisDir,
	sdtArchitectureDir,
	sdtDecisionsDir,
	sdtWorklogDir,
	sdtNotesDir,
	sdtQuestionsDir,
	sdtProposalsDir,
	sdtPromptsDir,
	sdtResearchDir,
	sdtTasksDir,
	sdtTrashDir,
	sdtTmpDir,
	sdtInstrDir,
	sdtCommandsDir,
	sdtScriptsDir,
	sdtRolesDir,
}

// agentVerifyChecks validates the AGENTS.md instruction contract:
//   - AGENTS.md exists and carries the instructions block
//   - every generated instruction file exists under context/instructions/
//   - no obsolete instruction files linger
//   - the context/ working directories exist (including context/scripts/)
//   - the instructions block references every generated instruction file
//
// Verifies against the current working directory.
func agentVerifyChecks() []ctxLintIssue {
	issues := []ctxLintIssue{}

	agents, err := os.ReadFile(agentTargetDefault) //#nosec G304 -- fixed repo-path check
	block := ""
	if err != nil {
		issues = append(issues, ctxLintIssue{Path: agentTargetDefault, Priority: ctxLintCritical, Message: "AGENTS.md not found (run sdt agent init)"})
	} else {
		block = string(agents)
		if !hasSection(block, agentSectionNameInstructions) {
			issues = append(issues, ctxLintIssue{Path: agentTargetDefault, Priority: ctxLintCritical, Message: "missing instructions block (run sdt agent init)"})
		}
	}

	// Only the selected profiles are required: a project that never opted into a domain
	// must not be told it is missing that domain's modules.
	profiles := currentProfiles()
	issues = agentVerifyInstructionFiles(issues, profiles)

	issues = append(issues, lintCommandsRegistry()...)

	for _, d := range agentVerifyWorkDirs {
		fi, err := os.Stat(d)
		if os.IsNotExist(err) {
			issues = append(issues, ctxLintIssue{Path: d, Priority: ctxLintWarning, Message: "missing work directory (run sdt agent init)"})
		} else if err != nil {
			issues = append(issues, ctxLintIssue{Path: d, Priority: ctxLintWarning, Message: err.Error()})
		} else if !fi.IsDir() {
			issues = append(issues, ctxLintIssue{Path: d, Priority: ctxLintWarning, Message: "exists and is not a directory"})
		}
	}

	if block != "" && hasSection(block, agentSectionNameInstructions) {
		for _, f := range instructionFilesFor(profiles, "", "") {
			want := "context/instructions/" + f.name
			if !strings.Contains(block, "`"+want+"`") {
				issues = append(issues, ctxLintIssue{Path: agentTargetDefault, Priority: ctxLintCritical, Message: "instructions block does not reference generated file " + want})
			}
		}
	}

	return issues
}

// agentVerifyInstructionFiles checks the generated instruction modules: every file the
// selected profiles require exists, none that the set no longer names lingers, and the
// bodies carry no H1. It is a function of its own because the parent's other checks are
// independent of it — the split keeps each one readable.
func agentVerifyInstructionFiles(issues []ctxLintIssue, profiles []string) []ctxLintIssue {
	// Only the selected profiles are required: a project that never opted into a
	// domain must not be told it is missing that domain's modules.
	for _, f := range instructionFilesFor(profiles, "", "") {
		path := filepath.Join(sdtInstrDir, f.name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintCritical, Message: "missing instruction file (run sdt agent init)"})
		} else if err != nil {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintCritical, Message: err.Error()})
		} else if data, rerr := os.ReadFile(path); rerr == nil { //#nosec G304 -- fixed corpus path
			// The document convention applies to the generated modules too: their
			// frontmatter `title` is the title, so a body `#` is that title written
			// twice. Surfaced as a WARNING, not a CRITICAL: this command fails on
			// CRITICALs alone, and an installed project whose generated copy predates
			// the template must not start failing a check merely by upgrading the
			// binary — the remedy is the same `sdt agent init --force` the obsolete
			// check names. Only the H1 finding is carried; a fenced-block defect in a
			// module is a template bug reported by the suite, not a state this
			// command should fail an installed project for.
			for _, issue := range lintMarkdownBody(path, frontmatterBody(data), false, false) {
				if !strings.Contains(issue.Message, "markdown H1") {
					continue
				}
				issue.Priority = ctxLintWarning
				issue.Message += " (generated module: fix the template, then re-run `sdt agent init --force`)"
				issues = append(issues, issue)
			}
		}
	}

	for _, name := range obsoleteInstructionFiles(profiles) {
		path := filepath.Join(sdtInstrDir, name)
		if _, err := os.Stat(path); err == nil {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: "obsolete instruction file (use sdt agent init --force to remove)"})
		}
	}

	return issues
}

// lintCommandsRegistry checks the trigger registry: every row resolves to a
// contract a reader can open, and a detail file is a document. The registry is
// the lookup surface, so a row that resolves to nothing is worse than a missing
// file — the agent would follow it.
func lintCommandsRegistry() []ctxLintIssue {
	var issues []ctxLintIssue
	if data, rerr := os.ReadFile(sdtCommandsIndex); rerr == nil { //#nosec G304 -- fixed registry path
		for _, row := range parseCommandRows(string(data)) {
			if row.Contract == "" || row.Contract == "—" {
				issues = append(issues, ctxLintIssue{Path: sdtCommandsIndex, Priority: ctxLintCritical, Message: "trigger >" + row.Trigger + " names no durable contract"})
				continue
			}
			if _, serr := os.Stat(row.Contract); serr != nil {
				issues = append(issues, ctxLintIssue{Path: sdtCommandsIndex, Priority: ctxLintCritical, Message: "trigger >" + row.Trigger + " names a contract that does not exist: " + row.Contract})
			}
			detail := filepath.Join(sdtCommandsDir, row.Trigger+sdtMarkdownExt)
			if ddata, derr := os.ReadFile(detail); derr == nil { //#nosec G304 -- fixed commands dir, registry row
				if !strings.HasPrefix(string(ddata), ctxFrontmatterDelim) {
					issues = append(issues, ctxLintIssue{Path: detail, Priority: ctxLintWarning, Message: "command detail file has no frontmatter"})
				}
			}
		}
	} else if os.IsNotExist(rerr) {
		issues = append(issues, ctxLintIssue{Path: sdtCommandsIndex, Priority: ctxLintWarning, Message: "no commands registry (run sdt agent init)"})
	}

	return issues
}

// agentVerifyCriticalError converts critical findings to the command's exit
// error without terminating the caller. Keeping this boundary pure lets tests
// assert the exact failure as a named Go test; the cobra Run callback remains
// responsible for enforcing the non-zero process exit.
func agentVerifyCriticalError(issues []ctxLintIssue) error {
	critical := 0
	for _, it := range issues {
		if it.Priority == ctxLintCritical {
			critical++
		}
	}
	if critical == 0 {
		return nil
	}
	return fmt.Errorf("%d CRITICAL issue(s)", critical)
}

var agentVerifyCmd = &cobra.Command{
	Use:   cmdVerify,
	Short: "Verify the AGENTS.md instruction contract",
	Long: `Validate that the AGENTS.md instruction contract matches the generated
instruction files:

  - AGENTS.md exists and carries the instructions block
  - every generated instruction file exists in context/instructions/
  - no generated instruction file opens with an H1 title (WARNING)
  - no obsolete instruction files linger
  - the context/ working directories exist (including context/scripts/)
  - the block references every generated instruction file

Exits non-zero when CRITICAL issues are found.

Examples:
  sdt agent verify
  sdt agent verify --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		issues := agentVerifyChecks()
		sort.Slice(issues, func(i, j int) bool {
			if issues[i].Priority != issues[j].Priority {
				prio := map[string]int{ctxLintCritical: 0, ctxLintWarning: 1}
				return prio[issues[i].Priority] < prio[issues[j].Priority]
			}
			return issues[i].Path < issues[j].Path
		})
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(issues, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(issues)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			if len(issues) == 0 {
				outputString(cmd, "OK: AGENTS.md instruction contract is valid\n")
			} else {
				for _, it := range issues {
					outputString(cmd, it.String()+"\n")
				}
			}
		}
		if err := agentVerifyCriticalError(issues); err != nil {
			exitWithError(cmd, err)
		}
	},
}
