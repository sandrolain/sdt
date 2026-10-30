package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/learn"
)

// ── sdt learn eval (behaviour cases) ─────────────────────────────────────────
//
// The grader and the scheduler are deterministic, so their contract can be pinned
// by cases rather than by prose. A case states an answer, the grade it must earn,
// and — when the schedule matters — the reason and interval it must produce. The
// cases live under context/learning/evals/ as plain YAML: no new document kind, no
// state written.

// learnEvalCase is one behaviour case.
type learnEvalCase struct {
	Name        string `json:"name" yaml:"name"`
	Rule        string `json:"rule,omitempty" yaml:"rule,omitempty"`
	ItemType    string `json:"item_type,omitempty" yaml:"item_type,omitempty"`
	Expected    string `json:"expected" yaml:"expected"`
	Answer      string `json:"answer" yaml:"answer"`
	Grade       string `json:"grade,omitempty" yaml:"grade,omitempty"`
	Hint        int    `json:"hint,omitempty" yaml:"hint,omitempty"`
	PriorReason string `json:"prior_reason,omitempty" yaml:"prior_reason,omitempty"`
	WantReason  string `json:"want_reason,omitempty" yaml:"want_reason,omitempty"`
	WantDays    int    `json:"want_days,omitempty" yaml:"want_days,omitempty"`
}

// learnEvalResult is one case's verdict.
type learnEvalResult struct {
	Case   string `json:"case" yaml:"case"`
	File   string `json:"file" yaml:"file"`
	OK     bool   `json:"ok" yaml:"ok"`
	Got    string `json:"got,omitempty" yaml:"got,omitempty"`
	Want   string `json:"want,omitempty" yaml:"want,omitempty"`
	Detail string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

var learnEvalCmd = &cobra.Command{
	Use:   "eval [file...]",
	Short: "Run the behaviour cases under context/learning/evals (read-only)",
	Long: `Run the deterministic grading and scheduling cases and report each verdict.

WHEN — after changing the grader, the schedule policy or a lesson's key, to prove
the contract still holds.
WHEN NOT — you want to grade one real answer: use sdt learn answer.

RETURNS — one row per case (ok or the expected and actual verdict), and a count.
A file holds a YAML list of cases; with no argument every ` + "`*.yaml`" + ` under
` + "`context/learning/evals/`" + ` runs. Exit 1 when a case fails. Reads only: no
learner state is written.

EXAMPLE — sdt learn eval
          sdt learn eval context/learning/evals/grading.yaml --format json`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		files, err := learnEvalFiles(args)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		results := make([]learnEvalResult, 0, len(files))
		for _, file := range files {
			cases, err := learnEvalRead(file)
			if err != nil {
				exitWithError(cmd, err)
				return
			}
			for _, c := range cases {
				results = append(results, learnEvalRun(file, c))
			}
		}
		failed := 0
		for _, r := range results {
			if !r.OK {
				failed++
			}
		}
		learnEmit(cmd, results, func() {
			if len(results) == 0 {
				outputString(cmd, "no eval cases under "+learnEvalDir()+"\n")
				return
			}
			for _, r := range results {
				status := "ok"
				if !r.OK {
					status = "FAIL"
				}
				line := fmt.Sprintf("%-4s %-30s %s", status, r.Case, r.File)
				if r.Detail != "" {
					line += " — " + r.Detail
				}
				outputString(cmd, line+"\n")
			}
			outputString(cmd, fmt.Sprintf("%d case(s), %d failed\n", len(results), failed))
		})
		if failed > 0 {
			exitWithError(cmd, fmt.Errorf("%d eval case(s) failed", failed))
		}
	},
}

// learnEvalDir is where the behaviour cases live.
func learnEvalDir() string {
	return filepath.Join(sdtLearningDir, "evals")
}

// learnEvalFiles resolves the cases to run: the named files, or every YAML file
// under the eval directory, sorted so a run is stable.
func learnEvalFiles(args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	entries, err := os.ReadDir(learnEvalDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".yaml", ".yml":
			files = append(files, filepath.Join(learnEvalDir(), e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

// learnEvalRead parses one case file.
func learnEvalRead(path string) ([]learnEvalCase, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- the case path is the caller's own argument
	if err != nil {
		return nil, fmt.Errorf("read eval cases: %w", err)
	}
	var cases []learnEvalCase
	if err := yaml.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("parse eval cases %s: %w", path, err)
	}
	return cases, nil
}

// learnEvalRun applies one case: the grade first, then the schedule when the case
// asks for one.
func learnEvalRun(file string, c learnEvalCase) learnEvalResult {
	r := learnEvalResult{Case: c.Name, File: file, OK: true}
	if strings.TrimSpace(c.Name) == "" {
		r.Case = "(unnamed)"
	}
	verdict := learn.GradeAnswer(c.Rule, c.Expected, c.Answer, c.ItemType)
	if c.Grade != "" && verdict.Grade != c.Grade {
		r.OK = false
		r.Got = verdict.Grade
		r.Want = c.Grade
		r.Detail = fmt.Sprintf("grade %s, want %s", verdict.Grade, c.Grade)
		return r
	}
	if c.WantReason == "" && c.WantDays == 0 {
		return r
	}
	days, reason := learn.Schedule(verdict.Grade, c.Hint, c.ItemType, c.PriorReason)
	if c.WantReason != "" && reason != c.WantReason {
		r.OK = false
		r.Got = reason
		r.Want = c.WantReason
		r.Detail = fmt.Sprintf("reason %s, want %s", reason, c.WantReason)
		return r
	}
	if c.WantDays != 0 && days != c.WantDays {
		r.OK = false
		r.Got = fmt.Sprintf("%d", days)
		r.Want = fmt.Sprintf("%d", c.WantDays)
		r.Detail = fmt.Sprintf("%d day(s), want %d", days, c.WantDays)
	}
	return r
}

func init() {
	learnCmd.AddCommand(learnEvalCmd)
}
