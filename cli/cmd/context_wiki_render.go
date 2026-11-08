package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/frontmatter"
)

// ── technical renders (sdt context wiki render) ──────────────────────────────
//
// A technical render is a dense projection of one wiki page, kept outside the
// index and outside the document kinds (decision 0034): the agent writes the
// prose, this verb owns the one machine fact — whether the render still matches
// the page it was generated from. The header line is the whole contract, and it
// is an HTML comment so a file under context/ with no `kind` cannot be read as a
// malformed document.

const (
	renderDir = "context/renders/technical"
	// RenderStatuses is the closed vocabulary of a render's freshness.
	renderFresh   = "fresh"
	renderStale   = "stale"
	renderMissing = "missing"
	renderOrphan  = "orphan"
)

// The material lane is the second shape of the same derived lane (decision/0038):
// material under context/renders/materials/<syllabus>/<concept>.<lang>.md, no kind
// and no frontmatter, its whole machine contract on the first line.
const (
	materialDir  = "context/renders/materials"
	materialLane = "material"
)

// renderHeaderRe matches the one line a render carries above its body.
var renderHeaderRe = regexp.MustCompile(`^<!-- generated from wiki/(\S+)@(\S+); regenerate, never edit -->$`)

// materialHeaderRe matches the one line a material carries: the concept it teaches,
// the syllabus it serves, the language, the rights position and the one claim id to
// digest pair per asserted claim.
var materialHeaderRe = regexp.MustCompile(
	`^<!-- material: concept=(\S+); syllabus=(\S+); lang=(\S+); classification=(\S+); claims=(\S+) -->$`)

// The three marker kinds a material's prose may carry (decision/0040). Each is an
// HTML comment so a reader skips it and a grep finds it.
var (
	materialClaimRe  = regexp.MustCompile(`<!--\s*claim=(\S+)#(c-[a-z0-9]+)\s+digest=(\S+)\s*-->`)
	materialLessonRe = regexp.MustCompile(`<!--\s*lesson=(\S+)#(\S+)\s*-->`)
	materialGlossRe  = regexp.MustCompile(`<!--\s*gloss=([^>]*?)\s*-->`)
	// materialObjectiveRe numbers an objective section (decision/0041).
	materialObjectiveRe = regexp.MustCompile(`<!--\s*objective=(\d+)\s*-->`)
	// materialCheckRe finds the one line after the exercise that names the page's
	// evaluation bank (decision/0046). It names items, never answers.
	materialCheckRe    = regexp.MustCompile(`(?m)^\*\*Verifica\.\*\*[^\n]*$`)
	materialAssessRe   = regexp.MustCompile(`<!--\s*assess=(\S+)\s+items=([^>]*?)\s*-->`)
)

// materialRank orders the four verdicts by severity, so two checks over one file
// report the worse one.
func materialRank(status string) int {
	switch status {
	case renderFresh:
		return 0
	case renderStale:
		return 1
	case renderMissing:
		return 2
	case renderOrphan:
		return 3
	default:
		return 4
	}
}

// materialProseIssue checks one material's prose against the page it binds and the
// lesson it cites, and returns the first failure with the verdict it earns. A
// material that marks nothing carries no binding, and silence is not evidence.
func materialProseIssue(concept, body string, page *contextwiki.Page) (string, string) {
	binding := map[string]string{}
	for _, claim := range page.Claims() {
		binding[claim.ID] = claim.Digest
	}
	marks := 0
	for _, match := range materialClaimRe.FindAllStringSubmatch(body, -1) {
		marks++
		owner, id, digest := strings.TrimPrefix(match[1], "wiki/"), match[2], match[3]
		if owner != concept {
			return renderStale, fmt.Sprintf("a claim marker names %s in a guide about %s", owner, concept)
		}
		live, ok := binding[id]
		if !ok {
			return renderMissing, fmt.Sprintf("claim %s is not on the page", id)
		}
		if live != digest {
			return renderStale, fmt.Sprintf("claim %s moved (marker %s, page %s)", id, digest, live)
		}
	}
	for _, match := range materialLessonRe.FindAllStringSubmatch(body, -1) {
		marks++
		owner, section := strings.TrimPrefix(match[1], "wiki/"), match[2]
		if owner != concept {
			return renderStale, fmt.Sprintf("a lesson marker names %s in a guide about %s", owner, concept)
		}
		ok, err := lessonHasSection(owner, section)
		if err != nil {
			return renderMissing, fmt.Sprintf("lesson for %s: %s", owner, err)
		}
		if !ok {
			return renderMissing, fmt.Sprintf("lesson %s carries no section %q", owner, section)
		}
	}
	for _, match := range materialGlossRe.FindAllStringSubmatch(body, -1) {
		marks++
		if strings.TrimSpace(match[1]) == "" {
			return renderStale, "a gloss marker names no term"
		}
	}
	if marks == 0 {
		return renderStale, "the prose carries no claim, lesson or gloss marker"
	}
	// The section contract (decisions 0041, 0045): one numbered marker per learning
	// objective, no number twice, no number the page does not have. The section
	// names and the order are editorial; only the count is mechanical.
	objectives := learningObjectivesOf(page.Frontmatter)
	seen := map[int]int{}
	for _, match := range materialObjectiveRe.FindAllStringSubmatch(body, -1) {
		n, convErr := strconv.Atoi(match[1])
		if convErr != nil || n < 1 || n > len(objectives) {
			return renderStale, fmt.Sprintf("objective marker %s is not one of the page's %d", match[1], len(objectives))
		}
		seen[n]++
	}
	for n := 1; n <= len(objectives); n++ {
		if seen[n] == 0 {
			return renderMissing, fmt.Sprintf("objective %d has no section", n)
		}
		if seen[n] > 1 {
			return renderStale, fmt.Sprintf("objective %d is counted %d times", n, seen[n])
		}
	}
	// The check line points at the item bank (decision/0046). A guide without one
	// passes: the exercise itself is optional, so a page with no exercise owes no
	// check. When the line is there, it must name the bank exactly.
	if status, detail := materialCheckIssue(concept, body); status != "" {
		return status, detail
	}
	return "", ""
}

// materialCheckIssue verifies the check line against the concept's item bank: every
// id it names must exist, and the line must name the bank's items and no others. A
// stale id, an id added to the bank and not to the line, and a line naming a bank
// the concept does not carry are each refused.
func materialCheckIssue(concept, body string) (string, string) {
	lines := materialCheckRe.FindAllString(body, -1)
	if len(lines) == 0 {
		return "", ""
	}
	if len(lines) > 1 {
		return renderStale, "the exercise carries more than one check line"
	}
	// The reader's sentence names no ids: they would mean nothing to a learner and
	// would tie the text to a bank that may be rewritten. The ids live in the marker
	// the gate reads, so the pointer is verifiable without being explained.
	marker := materialAssessRe.FindStringSubmatch(body)
	if marker == nil {
		return renderStale, "the check line carries no assess marker"
	}
	if owner := strings.TrimPrefix(marker[1], "wiki/"); owner != concept {
		return renderStale, fmt.Sprintf("the assess marker names %s in a guide about %s", owner, concept)
	}
	named := map[string]bool{}
	for _, id := range strings.Split(marker[2], ",") {
		if id = strings.TrimSpace(id); id != "" {
			named[id] = true
		}
	}
	items, ok := learnAssessmentItems(concept)
	if !ok {
		return renderMissing, "the check line names a bank the concept does not carry"
	}
	inBank := map[string]bool{}
	for _, item := range items {
		inBank[item.ID] = true
	}
	for id := range named {
		if !inBank[id] {
			return renderMissing, fmt.Sprintf("check item %s is not in the bank", id)
		}
	}
	if len(named) != len(inBank) {
		return renderStale, fmt.Sprintf("the check line names %d of the bank's %d items", len(named), len(inBank))
	}
	return "", ""
}

// lessonHasSection reports whether a concept's lesson carries the named section. The
// slug is the heading lowercased with whitespace turned into hyphens, which is how a
// marker writes it.
func lessonHasSection(concept, section string) (bool, error) {
	data, err := os.ReadFile(learnLessonPath(concept)) //#nosec G304 -- concept from the material header
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		heading, ok := strings.CutPrefix(strings.TrimRight(line, " \t"), "## ")
		if ok && materialSectionSlug(heading) == section {
			return true, nil
		}
	}
	return false, nil
}

// materialSectionSlug is the marker form of a lesson heading.
func materialSectionSlug(heading string) string {
	return strings.Join(strings.Fields(strings.ToLower(heading)), "-")
}

// renderStatuses returns the closed status vocabulary, freshest first.
func renderStatuses() []string {
	return []string{renderFresh, renderStale, renderMissing, renderOrphan}
}

// renderPath is where a page's render lives: the page id's slashes stay
// directories, so the path is the identity and cannot collide with a wiki id.
func renderPath(pageID string) string {
	return filepath.Join(renderDir, filepath.FromSlash(strings.TrimSpace(pageID))+".md")
}

// renderHeader is the line the verifier reads, and the one an agent must write.
func renderHeader(pageID, updated string) string {
	return fmt.Sprintf("<!-- generated from wiki/%s@%s; regenerate, never edit -->", pageID, updated)
}

// renderRow is one line of the report. `Lane` is empty for a technical render and
// `material` for the material lane, so an existing caller's JSON is unchanged.
// For a material, `Page` is the concept its header names, and the path-derived id
// only when the header cannot be read — the one case where the concept is unknown.
type renderRow struct {
	Page    string `json:"page" yaml:"page"`
	Path    string `json:"path" yaml:"path"`
	Status  string `json:"status" yaml:"status"`
	Lane    string `json:"lane,omitempty" yaml:"lane,omitempty"`
	Stamp   string `json:"stamp,omitempty" yaml:"stamp,omitempty"`
	Updated string `json:"updated,omitempty" yaml:"updated,omitempty"`
	Detail  string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

var contextWikiRenderCmd = &cobra.Command{
	Use:   "render [wiki-id...]",
	Short: "Report the freshness of the derived lane, or gate on it",
	Long: `Compare the derived lane with what it derives from: every technical render
(context/renders/technical/<page-id>.md) with the wiki page it was generated from,
and every material (context/renders/materials/<syllabus>/<concept>.<lang>.md) with
the claims its header pins.

STATUS — fresh (the stamp equals the page's updated), stale (it differs, or the
header cannot be read), missing (a page named on the command line has no render),
orphan (a render whose page does not exist). The walk finds the renders; a page
owes a render only when this command is asked about it.

A MATERIAL IS THE SECOND SHAPE OF THE SAME LANE (decision/0038): no kind, no
frontmatter, its contract on the first line — concept, syllabus, language,
classification and one claim id to digest pair per asserted claim. Its statuses
reuse the four words: fresh (every digest still matches), stale (one moved, or the
header cannot be read), missing (a claim the page no longer carries), orphan (no
page for the concept). It reports under ` + "`lane: material`" + ` in JSON.

THE PROSE IS CHECKED TOO (decision/0040): every claim marker must resolve to a live
claim at the digest it pins, every lesson marker to a section the lesson carries,
and every gloss marker to a non-empty term, and every objective marker to one of the
page's learning objectives, each number once (decision/0041); a guide whose prose
marks nothing is stale, because silence is not evidence. The check line that closes
an exercise (decisions 0046, 0047) carries an assess marker naming the concept's
item bank: every id it lists exists, and no item of the bank is left out. The ids sit
in the marker, not in the reader's sentence, which says only that a check exists. A
deck carries that one marker too, because a reader of the slides alone needs the
same pointer. A deck (<concept>.<lang>.slide.md)
is otherwise exempt:
its content is an outline and its speaker notes are prose, not assertions. The first
failure is reported, not a count.

EXIT — 0 by default: the report is a report. With ` + "`--check`" + ` it exits 1 when
anything in either lane is not fresh, so a caller that quotes a render or a material
gates on it first.

EXAMPLE — sdt context wiki render
          sdt context wiki render --check
          sdt context wiki render backend/auth --format json`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
		if err != nil {
			exitWithError(cmd, fmt.Errorf("load wiki: %w", err))
			return
		}
		rows, err := renderReport(bld, args)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		materials, err := materialReport(bld)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		rows = append(rows, materials...)
		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(rows, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(rows)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			if len(rows) == 0 {
				outputString(cmd, fmt.Sprintf("the derived lane is empty (%s, %s)\n", renderDir, materialDir))
			}
			for _, r := range rows {
				if r.Lane == materialLane {
					if r.Status == renderFresh {
						outputString(cmd, fmt.Sprintf("%s material %s\n", r.Status, r.Page))
						continue
					}
					outputString(cmd, fmt.Sprintf("%s material %s (%s)\n", r.Status, r.Page, r.Detail))
					continue
				}
				if r.Status == renderFresh {
					outputString(cmd, fmt.Sprintf("%s %s\n", r.Status, r.Page))
					continue
				}
				outputString(cmd, fmt.Sprintf("%s %s (%s → %s)\n",
					r.Status, r.Page, renderStampLabel(r.Stamp), renderStampLabel(r.Updated)))
			}
		}
		if getBoolFlag(cmd, "check", false) && !renderAllFresh(rows) {
			learnExitWith(fmt.Errorf("the derived lane is not fresh (%s, %s)", renderDir, materialDir), 1)
		}
	},
}

// renderStampLabel keeps an absent stamp visible in the text report.
func renderStampLabel(stamp string) string {
	if stamp == "" {
		return "no stamp"
	}
	return stamp
}

// renderAllFresh reports whether every row is fresh, the gate's condition.
func renderAllFresh(rows []renderRow) bool {
	for _, r := range rows {
		if r.Status != renderFresh {
			return false
		}
	}
	return true
}

// renderReport resolves the render set: every render on disk, plus one row for
// each page named on the command line (which is the only way `missing` can be
// reported — the lane never claims a page owes a render).
func renderReport(bld *contextwiki.Builder, pageIDs []string) ([]renderRow, error) {
	updated := pageUpdates(bld)
	rows := map[string]renderRow{}

	walked, err := walkRenders()
	if err != nil {
		return nil, err
	}
	for _, pageID := range walked {
		row := renderRow{Page: pageID, Path: filepath.ToSlash(renderPath(pageID))}
		stamp, headerErr := readRenderStamp(pageID)
		row.Stamp = stamp
		pageUpdated, ok := updated[pageID]
		switch {
		case !ok:
			row.Status = renderOrphan
		case headerErr != nil || stamp != pageUpdated:
			// A render that cannot prove its freshness is not fresh.
			row.Status = renderStale
			if ok {
				row.Updated = pageUpdated
			}
		default:
			row.Status = renderFresh
			row.Updated = pageUpdated
		}
		rows[pageID] = row
	}

	for _, raw := range pageIDs {
		pageID := strings.TrimSpace(raw)
		if pageID == "" {
			continue
		}
		page := bld.Resolve(pageID)
		if page == nil {
			return nil, fmt.Errorf("no wiki page matches %q: a render is a projection of a page, never of a guess", pageID)
		}
		if _, done := rows[page.ID]; done {
			continue
		}
		rows[page.ID] = renderRow{
			Page:    page.ID,
			Path:    filepath.ToSlash(renderPath(page.ID)),
			Status:  renderMissing,
			Updated: updated[page.ID],
		}
	}

	out := make([]renderRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Page < out[j].Page })
	return out, nil
}

// materialHeader is the one line a material's contract lives on: what it teaches,
// for which syllabus and language, under which rights position, and which claim at
// which digest each assertion derives from.
type materialHeader struct {
	Concept        string
	Syllabus       string
	Lang           string
	Classification string
	Claims         map[string]string
	Order          []string
}

// parseMaterialHeader reads the header line, or says why it cannot.
func parseMaterialHeader(line string) (materialHeader, error) {
	match := materialHeaderRe.FindStringSubmatch(strings.TrimSpace(line))
	if match == nil {
		return materialHeader{}, fmt.Errorf("first line is not a material header")
	}
	header := materialHeader{
		Concept:        strings.TrimPrefix(match[1], "wiki/"),
		Syllabus:       match[2],
		Lang:           match[3],
		Classification: match[4],
		Claims:         map[string]string{},
	}
	for _, pair := range strings.Split(match[5], ",") {
		id, digest, ok := strings.Cut(strings.TrimSpace(pair), "@")
		if !ok || id == "" || digest == "" {
			return materialHeader{}, fmt.Errorf("claim pair %q is not <id>@<digest>", pair)
		}
		header.Claims[id] = digest
		header.Order = append(header.Order, id)
	}
	return header, nil
}

// walkMaterials returns the path of every material on disk, relative to the lane.
func walkMaterials() ([]string, error) {
	var paths []string
	root := filepath.FromSlash(materialDir)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return fs.SkipDir
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), sdtMarkdownExt) {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", materialDir, err)
	}
	sort.Strings(paths)
	return paths, nil
}

// materialReport checks every material against the page it derives from. A
// material's identity is its header, so one it cannot prove is stale, never fresh.
// The worst condition wins: an id the page no longer carries is `missing`, a digest
// that moved is `stale`, and no page at all is `orphan`.
func materialReport(bld *contextwiki.Builder) ([]renderRow, error) {
	paths, err := walkMaterials()
	if err != nil {
		return nil, err
	}
	rows := make([]renderRow, 0, len(paths))
	for _, rel := range paths {
		row := renderRow{
			Page:   strings.TrimSuffix(rel, sdtMarkdownExt),
			Path:   filepath.ToSlash(filepath.Join(materialDir, filepath.FromSlash(rel))),
			Lane:   materialLane,
			Status: renderStale,
		}
		data, readErr := os.ReadFile(row.Path) //#nosec G304 -- path from the lane walk
		if readErr != nil {
			row.Detail = "cannot read the file"
			rows = append(rows, row)
			continue
		}
		first, rest, _ := strings.Cut(string(data), "\n")
		header, headerErr := parseMaterialHeader(first)
		if headerErr != nil {
			row.Detail = headerErr.Error()
			rows = append(rows, row)
			continue
		}
		row.Page, row.Stamp = header.Concept, header.Lang
		page := bld.Resolve(header.Concept)
		if page == nil {
			row.Status, row.Detail = renderOrphan, "no page "+header.Concept
			rows = append(rows, row)
			continue
		}
		live := map[string]string{}
		for _, claim := range page.Claims() {
			live[claim.ID] = claim.Digest
		}
		moved, gone := 0, 0
		for _, id := range header.Order {
			digest, ok := live[id]
			switch {
			case !ok:
				gone++
			case digest != header.Claims[id]:
				moved++
			}
		}
		switch {
		case gone > 0:
			row.Status, row.Detail = renderMissing, fmt.Sprintf("%d claim(s) the page no longer carries", gone)
		case moved > 0:
			row.Status, row.Detail = renderStale, fmt.Sprintf("%d claim(s) moved", moved)
		default:
			row.Status = renderFresh
		}
		// The header is the file's identity; the prose is what a reader acts on. A
		// prose that cannot prove its binding is not fresh, whatever the header says.
		// A deck is exempt: its content is an outline and its speaker notes are prose,
		// so the claim markers of a guide have no counterpart there (decision/0039).
		if strings.HasSuffix(rel, ".slide"+sdtMarkdownExt) {
			// A deck keeps one prose contract with its guide (decision/0046): the
			// check line, because a reader of the slides alone needs the same pointer.
			if status, detail := materialCheckIssue(header.Concept, rest); status != "" &&
				materialRank(status) > materialRank(row.Status) {
				row.Status, row.Detail = status, detail
			}
		} else if status, detail := materialProseIssue(header.Concept, rest, page); status != "" &&
			materialRank(status) > materialRank(row.Status) {
			row.Status, row.Detail = status, detail
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// pageUpdates maps every wiki page id to its frontmatter `updated`, the stamp a
// render is compared with. The shared Page carries no such field, and one
// consumer does not earn it a field there.
func pageUpdates(bld *contextwiki.Builder) map[string]string {
	out := make(map[string]string, len(bld.Pages))
	for _, page := range bld.Pages {
		block, err := frontmatter.Parse(page.Frontmatter)
		if err != nil {
			continue
		}
		if value, ok := block.Key("updated"); ok {
			out[page.ID] = value
		}
	}
	return out
}

// walkRenders returns the page id of every render on disk, sorted.
func walkRenders() ([]string, error) {
	var ids []string
	root := filepath.FromSlash(renderDir)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return fs.SkipDir
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), sdtMarkdownExt) {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		ids = append(ids, strings.TrimSuffix(filepath.ToSlash(rel), sdtMarkdownExt))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", renderDir, err)
	}
	sort.Strings(ids)
	return ids, nil
}

// readRenderStamp reads the stamp out of a render's first line.
func readRenderStamp(pageID string) (string, error) {
	data, err := os.ReadFile(renderPath(pageID)) //#nosec G304 -- derived from a path id, fixed root
	if err != nil {
		return "", err
	}
	first, _, _ := strings.Cut(string(data), "\n")
	match := renderHeaderRe.FindStringSubmatch(strings.TrimSpace(first))
	if match == nil {
		return "", fmt.Errorf("%s: first line is not a render header (%s)",
			renderPath(pageID), renderHeader(pageID, "<page-updated>"))
	}
	if match[1] != pageID {
		return "", fmt.Errorf("%s: header names %q, path names %q",
			renderPath(pageID), match[1], pageID)
	}
	return match[2], nil
}

func init() {
	contextWikiRenderCmd.Flags().Bool("check", false,
		"Exit 1 when anything in either derived lane is not fresh (the gate); without it the report always exits 0")
	contextWikiCmd.AddCommand(contextWikiRenderCmd)
}
