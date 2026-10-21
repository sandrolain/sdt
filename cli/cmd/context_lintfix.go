package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// `sdt context lint --fix` repairs the findings that are mechanical: an H1 the
// frontmatter title already carries, and a missing `objective` key that a parent
// document answers. Everything that needs a decision — the reading contract, a
// claim's evidence, a stale status — stays a finding.
//
// The default refuses terminal documents: a rule adopted today does not
// retroactively rewrite a closed plan or analysis. `--fix-terminal` opts in, as
// its own explicit act.

// lintFixTerminalStatuses are the states a document leaves behind: rewriting
// them is a corpus edit, not a normalisation.
var lintFixTerminalStatuses = map[string]bool{
	taskFileStatusCompleted: true,
	statusArchived:          true,
	ctxDocStatusSuperseded:  true,
	statusRejected:          true,
}

// lintFixInScope reports whether `--fix` may touch a document.
func lintFixInScope(content string, includeTerminal bool) bool {
	if includeTerminal {
		return true
	}
	return !lintFixTerminalStatuses[strings.TrimSpace(frontmatterField(content, "status"))]
}

// splitContextFrontmatter returns the frontmatter block (including its
// delimiters) and the body, so a fix rewrites one half without touching the
// other.
func splitContextFrontmatter(content string) (front, body string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content
	}
	if i := strings.Index(content[4:], "\n---\n"); i >= 0 {
		cut := 4 + i + len("\n---\n")
		return content[:cut], content[cut:]
	}
	return "", content
}

// lintFixRemoveH1 drops every level-1 heading outside a fenced block. The body
// starts at H2 because the frontmatter title is the document's title.
func lintFixRemoveH1(body string) (string, int) {
	lines := strings.Split(body, "\n")
	kept := make([]string, 0, len(lines))
	removed := 0
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			kept = append(kept, line)
			continue
		}
		if !inFence && strings.HasPrefix(line, "# ") {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	out := strings.Join(kept, "\n")
	if removed > 0 {
		// A removed title leaves the blank lines it separated; collapse the
		// leading run so the body still starts at its first section.
		out = strings.TrimLeft(out, "\n")
	}
	return out, removed
}

// lintFixObjective answers the `objective` a document is missing from its
// parent: a plan names its analysis, a task file its plan. An analysis has no
// parent, so the key stays a finding there — the fix never invents a grouping.
func lintFixObjective(content string) string {
	switch strings.TrimSpace(frontmatterField(content, "kind")) {
	case ctxTypePlan:
		if uid := strings.TrimSpace(frontmatterField(content, "analysis_id")); uid != "" {
			return lintObjectiveByUID(uid)
		}
	case ctxTypeTasks:
		for _, ref := range parseFrontmatterList(content, ctxFrontmatterLinks) {
			if !strings.HasPrefix(ref, "plan/") {
				continue
			}
			path := closeoutAbsRef(ref)
			//#nosec G304 -- corpus document named by the task file
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if obj := strings.TrimSpace(frontmatterField(string(data), "objective")); obj != "" {
				return obj
			}
		}
	}
	return ""
}

// lintObjectiveByUID finds the document carrying a uid and returns its
// objective.
func lintObjectiveByUID(uid string) string {
	for _, dir := range []string{sdtAnalysisDir, sdtPlanDir} {
		files, err := dirFiles(dir)
		if err != nil {
			continue
		}
		for _, path := range files {
			//#nosec G304 -- corpus listing
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := string(data)
			if strings.TrimSpace(frontmatterField(content, ctxFrontmatterUID)) != uid {
				continue
			}
			if obj := strings.TrimSpace(frontmatterField(content, "objective")); obj != "" {
				return obj
			}
		}
	}
	return ""
}

// lintFixDoc applies the mechanical normalisations to one document and returns
// a short description per change. A document with nothing to fix is written
// back untouched.
func lintFixDoc(path string, includeTerminal bool) ([]string, error) {
	//#nosec G304 -- corpus document selected by the caller
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	if !lintFixInScope(content, includeTerminal) {
		return nil, nil
	}
	// A map's `#` is its single root and a deck's `#` is a slide title: the lint
	// exempts both from the no-H1 rule, so the fix must exempt them too.
	if strings.HasSuffix(path, ".map.md") || contextwiki.IsSlideDoc(path) {
		return nil, nil
	}

	var changes []string
	front, body := splitContextFrontmatter(content)
	if fixed, removed := lintFixRemoveH1(body); removed > 0 {
		body = fixed
		changes = append(changes, fmt.Sprintf("removed %d H1 heading(s)", removed))
	}
	if strings.TrimSpace(frontmatterField(content, "objective")) == "" {
		if obj := lintFixObjective(content); obj != "" {
			if patched, ok := setFrontmatterFields(front+body, []frontmatterPatch{{key: "objective", value: obj}}); ok {
				front, body = splitContextFrontmatter(patched)
				changes = append(changes, "added objective "+obj)
			}
		}
	}
	if len(changes) == 0 {
		return nil, nil
	}
	//#nosec G306 G703 -- corpus document, 0644 like the rest
	if err := os.WriteFile(path, []byte(front+body), 0o644); err != nil {
		return nil, err
	}
	return changes, nil
}
