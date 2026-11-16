package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	gateStatusPassed   = "passed"
	gateStatusFailed   = "failed"
	gateStatusNotRun   = "not-run"
	gateStatusExcluded = "excluded"
)

// gateReport v1 is also the source of the human record: no second execution
// can turn an unexecuted check into a pass.
type gateReport struct {
	SchemaVersion  int              `json:"schema_version" yaml:"schema_version"`
	Revision       string           `json:"revision" yaml:"revision"`
	RevisionReason string           `json:"revision_reason,omitempty" yaml:"revision_reason,omitempty"`
	Dirty          bool             `json:"dirty" yaml:"dirty"`
	ConfigSHA256   string           `json:"config_sha256" yaml:"config_sha256"`
	Started        time.Time        `json:"started" yaml:"started"`
	Status         string           `json:"status" yaml:"status"`
	Configuration  []GateStep       `json:"configuration" yaml:"configuration"`
	Results        []gateStepResult `json:"results" yaml:"results"`
}

func newGateReport(root string, steps []GateStep) (gateReport, error) {
	//#nosec G304 -- fixed configuration filename in the discovered project root
	data, err := os.ReadFile(filepath.Join(root, sdtConfigFile))
	if err != nil {
		return gateReport{}, err
	}
	r := gateReport{SchemaVersion: 1, Started: contextNow().UTC(), Status: gateStatusExcluded,
		ConfigSHA256:  fmt.Sprintf("%x", sha256.Sum256(data)),
		Configuration: append([]GateStep{}, steps...), Results: []gateStepResult{}}
	git := exec.Command("git", "rev-parse", "HEAD")
	git.Dir = root
	if out, err := git.Output(); err == nil {
		r.Revision = strings.TrimSpace(string(out))
	} else {
		r.Revision = "unavailable"
		r.RevisionReason = err.Error()
	}
	git = exec.Command("git", "status", "--porcelain")
	git.Dir = root
	if out, err := git.Output(); err == nil {
		r.Dirty = len(out) > 0
	} else if r.RevisionReason == "" {
		r.RevisionReason = err.Error()
	}
	return r, nil
}

func (r *gateReport) execute(ctx context.Context, root string) string {
	failed := ""
	for _, step := range r.Configuration {
		result := gateStepResult{Step: step.Name, Status: gateStatusNotRun, Reason: "stopped after failure at " + failed}
		switch {
		case strings.TrimSpace(step.Exclude) != "":
			result.Status, result.Reason = gateStatusExcluded, step.Exclude
		case failed != "":
		case ctx.Err() != nil:
			result.Reason = ctx.Err().Error()
			r.Status = gateStatusFailed
			failed = step.Name
		default:
			started := contextNow()
			//#nosec G204 -- the project owner declares this shell command in .sdt.yaml
			command := exec.CommandContext(ctx, "/bin/sh", "-c", step.Run)
			command.Dir = root
			// Descendants holding the pipe open must not prevent an interrupted
			// gate from writing the unknown outcomes of the remaining checks.
			command.WaitDelay = time.Second
			out, err := command.CombinedOutput()
			result.Duration = contextNow().Sub(started)
			result.Status, result.Reason = gateStatusPassed, "command exited successfully"
			if err != nil {
				result.Status, result.Reason = gateStatusFailed, err.Error()
				if ctx.Err() != nil {
					result.Reason = ctx.Err().Error()
				}
				result.Detail, result.Log = gateFailureDetail(step.Name, out)
				failed = step.Name
				r.Status = gateStatusFailed
			} else if r.Status != gateStatusFailed {
				r.Status = gateStatusPassed
			}
		}
		r.Results = append(r.Results, result)
	}
	if r.Status == gateStatusPassed {
		for _, result := range r.Results {
			if result.Status == gateStatusExcluded {
				r.Status = gateStatusExcluded
				break
			}
		}
	}
	return failed
}

func (r gateReport) validate() error {
	if len(r.Results) != len(r.Configuration) {
		return fmt.Errorf("incomplete gate report")
	}
	seen := map[string]bool{}
	for i, result := range r.Results {
		if seen[result.Step] || result.Step != r.Configuration[i].Name || result.Reason == "" {
			return fmt.Errorf("invalid gate result for %q", result.Step)
		}
		seen[result.Step] = true
		switch result.Status {
		case gateStatusPassed, gateStatusFailed, gateStatusNotRun, gateStatusExcluded:
		default:
			return fmt.Errorf("invalid gate status %q", result.Status)
		}
		if r.Status == gateStatusPassed && (result.Status == gateStatusFailed || result.Status == gateStatusNotRun) {
			return fmt.Errorf("incomplete or failed gate cannot pass")
		}
	}
	return nil
}
