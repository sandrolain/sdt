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
	sdtRFCsDir,
	sdtPromptsDir,
	sdtTasksDir,
	sdtArchiveDir,
	sdtTmpDir,
	sdtInstrDir,
	sdtScriptsDir,
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

	for _, f := range instructionFiles("", "") {
		path := filepath.Join(sdtInstrDir, f.name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintCritical, Message: "missing instruction file (run sdt agent init)"})
		} else if err != nil {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintCritical, Message: err.Error()})
		}
	}

	for _, name := range obsoleteInstructionFiles {
		path := filepath.Join(sdtInstrDir, name)
		if _, err := os.Stat(path); err == nil {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: "obsolete instruction file (use sdt agent init --force to remove)"})
		}
	}

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
		for _, f := range instructionFiles("", "") {
			want := "context/instructions/" + f.name
			if !strings.Contains(block, "`"+want+"`") {
				issues = append(issues, ctxLintIssue{Path: agentTargetDefault, Priority: ctxLintCritical, Message: "instructions block does not reference generated file " + want})
			}
		}
	}

	return issues
}

var agentVerifyCmd = &cobra.Command{
	Use:   cmdVerify,
	Short: "Verify the AGENTS.md instruction contract",
	Long: `Validate that the AGENTS.md instruction contract matches the generated
instruction files:

  - AGENTS.md exists and carries the instructions block
  - every generated instruction file exists in context/instructions/
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
		critical := 0
		for _, it := range issues {
			if it.Priority == ctxLintCritical {
				critical++
			}
		}
		if critical > 0 {
			exitWithError(cmd, fmt.Errorf("%d CRITICAL issue(s)", critical))
		}
	},
}
