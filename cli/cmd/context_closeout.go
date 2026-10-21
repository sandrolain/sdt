package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The crystallized closeout is generated from the ledger: the checklist states,
// the review verdicts and the gate result are facts the plan already holds, so
// asking an agent to retype them produced five hand-written copies that drifted
// from the plan. What no command can know — the files touched, the local
// decisions, the lessons, the deviations — stays a labelled slot for the agent.
//
// `sdt context close` writes the block; a rerun regenerates the derived fields
// and keeps the human ones, so a second close is a refresh and not a duplicate.

const (
	closeoutHeadingPrefix = "### Closeout — "
	closeoutGateSection   = "gate"
)

// closeoutItem is one checklist line of the ledger and its recorded state.
type closeoutItem struct {
	Text string
	Done bool
}

// closeoutFacts is the part of a closeout the ledger can answer.
type closeoutFacts struct {
	PlanRef  string // corpus-relative plan path
	TaskRef  string // corpus-relative task path, empty for an unsplit plan
	Slug     string // identity in the heading
	Items    []closeoutItem
	Verdicts []string
	Gate     string
}

// closeoutParseLedger reads the checklist, the `## Review` verdicts and the
// `### Gate` result from a plan or task file.
func closeoutParseLedger(path string) (closeoutFacts, error) {
	//#nosec G304 -- corpus document resolved by the caller
	data, err := os.ReadFile(path)
	if err != nil {
		return closeoutFacts{}, err
	}
	content := string(data)
	if i := strings.Index(content, "\n---\n"); strings.HasPrefix(content, "---\n") && i >= 0 {
		content = content[i+5:]
	}

	items, verdicts, gate := closeoutScanLedger(content)
	return closeoutFacts{Items: items, Verdicts: verdicts, Gate: gate}, nil
}

// closeoutScanLedger walks the body once: a checklist line belongs to the
// outcome, `## Review` supplies the verdicts and `### Gate` the last run. Fenced
// blocks are prose examples, never claims.
func closeoutScanLedger(body string) ([]closeoutItem, []string, string) {
	var items []closeoutItem
	var verdicts []string
	var gate string
	section := ""
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "### Gate"):
			section = closeoutGateSection
			continue
		case strings.HasPrefix(trimmed, "## Review"):
			section = ctxReviewVerb
			continue
		case strings.HasPrefix(trimmed, "## "), strings.HasPrefix(trimmed, "### "):
			section = ""
		}
		if m := closeoutItemRegexp.FindStringSubmatch(trimmed); m != nil {
			items = append(items, closeoutItem{Text: strings.TrimSpace(m[2]), Done: strings.ToLower(m[1]) == "x"})
			continue
		}
		if trimmed == "" {
			continue
		}
		switch section {
		case ctxReviewVerb:
			verdicts = append(verdicts, strings.TrimPrefix(trimmed, "- "))
		case closeoutGateSection:
			// A recorded run is the strongest evidence the section holds, and
			// the last one wins; without one, the first line is the best the
			// ledger has.
			line := strings.TrimPrefix(trimmed, "- ")
			if strings.Contains(line, "(sdt agent gate)") {
				gate = line
			} else if gate == "" {
				gate = line
			}
		}
	}
	return items, verdicts, gate
}

var closeoutItemRegexp = regexp.MustCompile(`^- \[( |x|X)\] (.+)$`)

// closeoutAnchorRegexp matches the CLI-owned checklist id the plan carries, so
// the record quotes the item without the editor's address.
var closeoutAnchorRegexp = regexp.MustCompile(`\s*<!-- c[0-9]+ -->$`)

// closeoutStripAnchor removes the checklist anchor from an item's text.
func closeoutStripAnchor(text string) string {
	return strings.TrimSpace(closeoutAnchorRegexp.ReplaceAllString(text, ""))
}

// closeoutSlug is the identity used in the heading: the plan's own slug without
// its timestamp, plus the task's suffix on a split plan.
func closeoutSlug(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), sdtMarkdownExt)
	if m := regexp.MustCompile(`^[0-9]{8}-[0-9]{6}-`).FindString(base); m != "" {
		base = strings.TrimPrefix(base, m)
	}
	return base
}

// closeoutHumanFields are the lines only the agent can fill. A rerun carries
// them over; a first run leaves the marker that says so.
var closeoutHumanFields = []struct {
	Label  string
	Marker string
}{
	{"Files affected", "fill with the paths touched by the delivery"},
	{"Decisions", "small local ones here; an architectural one becomes a decision record, linked"},
	{"Lessons / Do-Not-Repeat candidates", "→ the `context/notes/` lessons note, linked"},
	{"Signal review", "routed capture per *Signal review*, or `none`"},
	{"Deviations / blockers", "explicit, with resolution or open status"},
}

// closeoutRenderBlock builds the closeout section. `carried` supplies the human
// field values a previous block held; a missing key keeps its marker.
func closeoutFactsRender(f closeoutFacts, date string, carried map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s — %s\n\n", closeoutHeadingPrefix, date, f.Slug)
	fmt.Fprintf(&b, "- **Plan ref**: `%s` — the entry's identity\n", f.PlanRef)
	if f.TaskRef != "" {
		fmt.Fprintf(&b, "- **Task ref**: `%s` — the identity of this stream\n", f.TaskRef)
	} else {
		b.WriteString("- **Task ref** (split only): — (this plan is not split)\n")
	}

	b.WriteString("- **Outcome** (claims are `expected`: the ledger records the item as\n")
	b.WriteString("  done, not that it was run — the verdicts below carry the verification):\n")
	done := 0
	for _, it := range f.Items {
		if !it.Done {
			continue
		}
		done++
		// The ledger says the item is done, not that it was verified: `expected`
		// is the honest claim, and the review block below carries what was run.
		fmt.Fprintf(&b, "  - [x] %s — **expected**\n", closeoutStripAnchor(it.Text))
	}
	if done == 0 {
		b.WriteString("  - _the ledger holds no completed item_\n")
	}
	open := len(f.Items) - done
	if open > 0 {
		fmt.Fprintf(&b, "  - **open**: %d item(s) not done in the ledger — see the plan\n", open)
	}

	for _, fld := range closeoutHumanFields {
		if v := strings.TrimSpace(carried[fld.Label]); v != "" {
			fmt.Fprintf(&b, "- **%s**: %s\n", fld.Label, v)
		} else {
			fmt.Fprintf(&b, "- **%s**: _%s_\n", fld.Label, fld.Marker)
		}
	}

	b.WriteString("- **Review verdicts**:\n")
	if len(f.Verdicts) == 0 {
		b.WriteString("  - _no review findings recorded_\n")
	}
	for _, v := range f.Verdicts {
		fmt.Fprintf(&b, "  - %s\n", v)
	}
	if f.Gate != "" {
		fmt.Fprintf(&b, "- **Gate**: %s\n", f.Gate)
	}
	fmt.Fprintf(&b, "- **Provenance**: `%s` (checklist, `## Review`), generated by `sdt context close` on %s\n",
		f.PlanRef, date)
	return b.String()
}

// closeoutFindSection locates the closeout for a slug: its heading carries the
// slug, so a plan and each of its task files get one block apiece. It returns
// the line range and the date the original heading recorded.
func closeoutFindSection(content, slug string) (start, end int, date string) {
	lines := strings.Split(content, "\n")
	start, end = -1, len(lines)
	for i, line := range lines {
		if start < 0 {
			if !strings.HasPrefix(line, closeoutHeadingPrefix) || !strings.HasSuffix(strings.TrimSpace(line), slug) {
				continue
			}
			start = i
			if len(line) > len(closeoutHeadingPrefix) {
				rest := strings.TrimPrefix(line, closeoutHeadingPrefix)
				if idx := strings.Index(rest, " — "); idx > 0 {
					date = strings.TrimSpace(rest[:idx])
				}
			}
			continue
		}
		if strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "## ") {
			end = i
			break
		}
	}
	return start, end, date
}

// closeoutCarriedFields reads the human lines a previous block holds, so a
// regeneration keeps what the agent wrote and refreshes only the derived part.
func closeoutCarriedFields(block string) map[string]string {
	carried := map[string]string{}
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- **") {
			continue
		}
		rest := strings.TrimPrefix(trimmed, "- **")
		idx := strings.Index(rest, "**:")
		if idx < 0 {
			continue
		}
		label, value := rest[:idx], strings.TrimSpace(rest[idx+3:])
		if value == "" || (strings.HasPrefix(value, "_") && strings.HasSuffix(value, "_")) {
			continue
		}
		carried[label] = value
	}
	return carried
}

// closeoutWrite records the closeout of the document just closed: it finds or
// creates the worklog entry that carries the plan identity, replaces the block
// for that identity when one exists, and reports the path so the caller can say
// where the record went. A non-plan document returns "" and no error: the
// closeout belongs to the ledger, not to every close.
func closeoutWrite(doc ctxResolvedDoc) (string, error) {
	planPath, taskPath := doc.Path, ""
	switch {
	case doc.Type.kind == ctxTypeTasks:
		taskPath = doc.Path
		planPath = closeoutPlanRef(doc.Path)
		if planPath == "" {
			return "", nil
		}
	case doc.Type.kind != ctxTypePlan:
		return "", nil
	}

	facts, err := closeoutParseLedger(planPath)
	if err != nil {
		return "", err
	}
	facts.PlanRef = closeoutRef(planPath)
	facts.Slug = closeoutSlug(planPath)
	var taskFacts closeoutFacts
	if taskPath != "" {
		taskFacts, err = closeoutParseLedger(taskPath)
		if err != nil {
			return "", err
		}
		facts.TaskRef = closeoutRef(taskPath)
		facts.Items = taskFacts.Items
		facts.Verdicts = taskFacts.Verdicts
		facts.Gate = taskFacts.Gate
		// The task slug already carries the plan's name, so it is the identity of
		// the stream on its own: one block per task file, no doubled prefix.
		facts.Slug = closeoutSlug(taskPath)
	}

	entry, err := closeoutWorklogEntry(facts.PlanRef)
	if err != nil {
		return "", err
	}
	//#nosec G304 -- corpus document resolved above
	data, err := os.ReadFile(entry)
	if err != nil {
		return "", err
	}
	content := string(data)

	date := contextNow().UTC().Format("2006-01-02")
	carried := map[string]string{}
	start, end, existingDate := closeoutFindSection(content, facts.Slug)
	block := ""
	if start >= 0 {
		block = strings.Join(strings.Split(content, "\n")[start:end], "\n")
		carried = closeoutCarriedFields(block)
		if existingDate != "" {
			date = existingDate
		}
	}
	rendered := closeoutFactsRender(facts, date, carried)

	lines := strings.Split(content, "\n")
	if start >= 0 {
		lines = append(lines[:start], append(strings.Split(strings.TrimRight(rendered, "\n"), "\n"), lines[end:]...)...)
	} else {
		content = strings.TrimRight(content, "\n") + "\n\n" + rendered
		lines = strings.Split(content, "\n")
	}
	content = strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	//#nosec G306 G703 -- corpus path, 0644 like every work file
	if err := os.WriteFile(entry, []byte(content), 0o644); err != nil {
		return "", err
	}
	closeoutTouch(entry)
	return entry, nil
}

// closeoutPlanRef reads the plan a task file belongs to from its `links`.
func closeoutPlanRef(taskPath string) string {
	//#nosec G304 -- corpus document
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return ""
	}
	for _, ref := range parseFrontmatterList(string(data), ctxFrontmatterLinks) {
		if strings.HasPrefix(ref, "plan/") {
			if p := closeoutAbsRef(ref); p != "" {
				return p
			}
		}
	}
	return ""
}

// closeoutWorklogEntry finds the worklog entry that carries a plan identity, or
// creates one: the closeout lives beside the plan's own record, never in a
// second document per close.
func closeoutWorklogEntry(planRef string) (string, error) {
	files, err := dirFiles(sdtWorklogDir)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	for _, path := range files {
		//#nosec G304 -- corpus directory listing
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, ref := range parseFrontmatterList(string(data), ctxFrontmatterLinks) {
			if ref == planRef {
				return path, nil
			}
		}
	}

	slug := "closeout-" + closeoutSlug(planRef)
	path, err := contextPath(ctxTypeWorklog, slug, "", "", "")
	if err != nil {
		return "", err
	}
	project := ""
	if cfg, cfgErr := findProjectConfig(); cfgErr == nil && cfg != nil {
		project = cfg.Project
	}
	created := contextNow().UTC().Format(time.RFC3339)
	content := contextFrontmatter(ctxTypeWorklog,
		"", "Crystallized closeout for "+planRef, "", project, "", created,
		"", slug, "", "", "", "", nil, nil, nil)
	content = injectReferenceList(content, ctxFrontmatterLinks, []string{planRef})
	content = injectReferenceList(content, ctxFrontmatterSources, []string{planRef})
	content += "\n## Closeouts\n\n_Generated by `sdt context close`: one block per plan identity._\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	//#nosec G306 G703 -- corpus path, 0644 like every work file
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// closeoutTouch refreshes `updated` on the entry so the corpus index sees the
// close as a change, without rewriting any other field.
func closeoutTouch(path string) {
	//#nosec G304 -- corpus document
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	content, ok := setFrontmatterFields(string(data), []frontmatterPatch{{
		key:   statusUpdated,
		value: contextNow().UTC().Format(time.RFC3339),
	}})
	if !ok {
		return
	}
	//#nosec G306 G703 -- corpus path
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return
	}
}

// closeoutRef names a corpus document the way frontmatter does: relative to
// `context/`, so `plan/<file>` rather than the working path.
func closeoutRef(path string) string {
	p := filepath.ToSlash(path)
	if i := strings.Index(p, sdtWorkDir+"/"); i >= 0 {
		return p[i+len(sdtWorkDir)+1:]
	}
	return p
}

// closeoutAbsRef resolves a frontmatter ref (`plan/<file>`) to a path.
func closeoutAbsRef(ref string) string {
	if ref == "" {
		return ""
	}
	return filepath.Join(sdtWorkDir, filepath.FromSlash(ref))
}
