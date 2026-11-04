package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/learn"
)

// ── sdt learn lesson (the composed bundle) ────────────────────────────────────
//
// A lesson is a document; this verb is how a tutor or a portal reads it without
// re-implementing the contract. `--validate` is the cross-file half of the check
// `lintLesson` does on shape alone: the claim an item cites must exist on the
// page, the objective it points at must exist in the block, and every objective
// must be represented.

// learnLessonItem is one assessment item, as the bundle carries it. The first
// eight fields are the base table; the last three are optional columns that state
// how the item is graded, where it came from and when its key was frozen.
type learnLessonItem struct {
	ID         string `json:"id" yaml:"id"`
	Objective  int    `json:"objective" yaml:"objective"`
	Type       string `json:"type" yaml:"type"`
	Prompt     string `json:"prompt" yaml:"prompt"`
	Expected   string `json:"expected" yaml:"expected"`
	Rubric     string `json:"rubric" yaml:"rubric"`
	Distractor string `json:"distractor" yaml:"distractor"`
	Claim      string `json:"claim" yaml:"claim"`
	Grading    string `json:"grading" yaml:"grading"`
	Provenance string `json:"provenance" yaml:"provenance"`
	Frozen     string `json:"frozen,omitempty" yaml:"frozen,omitempty"`
	// Hash is the claim digest the item was written against: when the claim moves,
	// the item is stale. Absent means the item never pinned one.
	Hash string `json:"hash,omitempty" yaml:"hash,omitempty"`
	// KeyHash is the stored freeze digest of the four cells that decide a verdict.
	// Absent means the item was written before the freeze existed; the read path
	// then publishes the value the cell should carry.
	KeyHash string `json:"key_hash,omitempty" yaml:"key_hash,omitempty"`
	// KeyDigest is the digest computed from the row as it now reads. It is what the
	// author copies into the `key_hash` cell and what a mismatched cell is
	// compared with, so no caller ever recomputes it a second way.
	KeyDigest string `json:"key_digest,omitempty" yaml:"key_digest,omitempty"`
}

// learnLessonGradingRules is the closed grading vocabulary. `auto` picks a rule
// from the item's shape; `manual` never grades.
var learnLessonGradingRules = learn.GradingRules()

// learnLessonProvenances is the closed provenance vocabulary: where the expected
// answer came from, never how good it is.
var learnLessonProvenances = []string{"material", "ai_generated", "mixed", "unknown"}

// learnLessonOptionalColumns is the optional trailing header, in order. A lesson
// that carries none of them is still valid: every optional value has a default.
var learnLessonOptionalColumns = []string{"grading", "provenance", "frozen", "hash", "key_hash"}

// Defaults an absent optional cell takes.
const (
	learnGradingAuto       = learn.RuleAuto
	learnProvenanceUnknown = "unknown"
)

// learnWorkedExample is the example section, with the span it was re-expressed
// from: the citation is part of the value, not decoration.
type learnWorkedExample struct {
	SourceSpan string `json:"source_span,omitempty" yaml:"source_span,omitempty"`
	Text       string `json:"text" yaml:"text"`
}

// learnLessonClaim is the page claim an item can cite, with the digest the
// `hash` cell carries. The bundle publishes it so an author fills the cell from
// the same command that composes the lesson.
type learnLessonClaim struct {
	ID     string `json:"id" yaml:"id"`
	Number int    `json:"number" yaml:"number"`
	Digest string `json:"digest" yaml:"digest"`
	Text   string `json:"text,omitempty" yaml:"text,omitempty"`
}

// learnLessonBundle is the composed lesson: what a portal renders and asks.
type learnLessonBundle struct {
	Concept       string             `json:"concept" yaml:"concept"`
	Title         string             `json:"title,omitempty" yaml:"title,omitempty"`
	Objectives    []string           `json:"objectives" yaml:"objectives"`
	Difficulty    string             `json:"difficulty,omitempty" yaml:"difficulty,omitempty"`
	Minutes       int                `json:"minutes,omitempty" yaml:"minutes,omitempty"`
	Prerequisites []string           `json:"prerequisites,omitempty" yaml:"prerequisites,omitempty"`
	Example       learnWorkedExample `json:"worked_example" yaml:"worked_example"`
	Items         []learnLessonItem  `json:"items" yaml:"items"`
	Claims        []learnLessonClaim `json:"claims,omitempty" yaml:"claims,omitempty"`
	Transfer      string             `json:"transfer,omitempty" yaml:"transfer,omitempty"`
	Missing       []string           `json:"missing,omitempty" yaml:"missing,omitempty"`
	_             struct{}           `json:"-" yaml:"-"`
}

// learnLessonClaimsOf publishes the page's claims with the digest the validator
// compares, so the author copies a value rather than computing one.
func learnLessonClaimsOf(p *contextwiki.Page) []learnLessonClaim {
	claims := p.Claims()
	out := make([]learnLessonClaim, 0, len(claims))
	for _, c := range claims {
		out = append(out, learnLessonClaim{ID: c.ID, Number: c.Number, Digest: c.Digest, Text: c.Text})
	}
	return out
}

// ctxLessonSpanRegexp finds the provenance of a worked example.
var ctxLessonSpanRegexp = regexp.MustCompile(`refs/[^\s@]+@[0-9a-f]{8}:[0-9]+-[0-9]+`)

var learnLessonCmd = &cobra.Command{
	Use:   "lesson [concept]",
	Short: "Compose a lesson from its page and its lesson document (read-only)",
	Long: `Compose the teaching bundle for a concept: objectives, pacing, prerequisites,
the worked example with its span, and the assessment items.

WHEN — a tutor or a portal needs the lesson as data, or the content must be
checked before it is used.
WHEN NOT — you want to know whether the concept can be taught at all: ` + "`sdt learn check`" + `.
WHEN NOT — you want the page itself: open the wiki page.

RETURNS — the bundle (text, or --format json for a caller). ` + "`--validate`" + ` walks
every lesson and reports what would make it unusable: a missing page, a claim the
page does not carry, an objective index out of range, an objective with no item,
an empty expected or rubric, a duplicate item id. ` + "`--coverage`" + ` prints the
objective to item matrix. Exit 1 on any finding. Reads only.

EXAMPLE — sdt learn lesson architecture/four-key-metrics --format json
          sdt learn lesson --validate
          sdt learn lesson --coverage`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bld := loadLearnWiki(cmd)
		switch {
		case getBoolFlag(cmd, "validate", false):
			issues, counts := learnValidateAll(bld)
			learnEmit(cmd, issues, func() {
				if len(issues) == 0 {
					outputString(cmd, "every lesson is usable\n")
				}
				for _, i := range issues {
					outputString(cmd, i+"\n")
				}
				outputString(cmd, counts.line()+"\n")
			})
			if len(issues) > 0 {
				exitWithError(cmd, fmt.Errorf("%d lesson finding(s)", len(issues)))
			}
		case getBoolFlag(cmd, "coverage", false):
			rows := learnCoverage(bld)
			learnEmit(cmd, rows, func() {
				for _, r := range rows {
					outputString(cmd, r.line()+"\n")
				}
			})
		default:
			if len(args) != 1 {
				exitWithError(cmd, fmt.Errorf("a concept is required, use sdt learn lesson <concept>"))
				return
			}
			page := learnResolveConcept(bld, args[0])
			if page == nil {
				exitWithError(cmd, fmt.Errorf("no wiki page matches %q", args[0]))
				return
			}
			bundle := learnBundleOf(bld, page)
			learnEmit(cmd, bundle, func() {
				outputString(cmd, bundle.text())
				learnSuggestNext(cmd, loadLearnState(cmd), page.ID)
			})
		}
	},
}

// learnCoverageRow is one line of the objective-to-item matrix.
type learnCoverageRow struct {
	Concept    string `json:"concept" yaml:"concept"`
	Objectives int    `json:"objectives" yaml:"objectives"`
	Items      int    `json:"items" yaml:"items"`
	Uncovered  []int  `json:"uncovered,omitempty" yaml:"uncovered,omitempty"`
}

func (r learnCoverageRow) line() string {
	state := "covered"
	if len(r.Uncovered) > 0 {
		state = fmt.Sprintf("uncovered %v", r.Uncovered)
	}
	return fmt.Sprintf("%-40s objectives=%d items=%d %s", r.Concept, r.Objectives, r.Items, state)
}

// learnBundleOf composes one lesson.
func learnBundleOf(bld *contextwiki.Builder, p *contextwiki.Page) learnLessonBundle {
	b := learnLessonBundle{
		Concept:       p.ID,
		Title:         p.Title,
		Objectives:    learningObjectivesOf(p.Frontmatter),
		Prerequisites: learnPrerequisites(bld, p),
		Items:         []learnLessonItem{},
	}
	if block, ok := learningBlockOf(p.Frontmatter); ok {
		if d, ok := block["difficulty"].(string); ok {
			b.Difficulty = d
		}
		if m, ok := block["minutes"]; ok {
			if n, err := strconv.Atoi(fmt.Sprintf("%v", m)); err == nil {
				b.Minutes = n
			}
		}
	}
	data, err := os.ReadFile(learnLessonPath(p.ID)) //#nosec G304 -- path from the page id
	if err != nil {
		b.Missing = []string{fmt.Sprintf("no lesson document at %s", learnLessonPath(p.ID))}
		return b
	}
	body := documentBody(string(data))
	_, sections := markdownSections(body)
	example := sections["Worked example"]
	b.Example = learnWorkedExample{
		SourceSpan: ctxLessonSpanRegexp.FindString(example),
		Text:       strings.TrimSpace(example),
	}
	b.Transfer = strings.TrimSpace(sections["Transfer"])
	b.Claims = learnLessonClaimsOf(p)
	// The items are the concept's bank, not the lesson's: the lesson is what a
	// learner reads, the bank is what a grader may see.
	items, hasBank := learnAssessmentItems(p.ID)
	b.Items = items
	switch {
	case !hasBank:
		b.Missing = append(b.Missing, fmt.Sprintf("no assessment document at %s (the item bank of %q)", learnAssessmentPath(p.ID), p.ID))
	case len(b.Items) == 0:
		b.Missing = append(b.Missing, "the Items table has no readable item")
	}
	return b
}

// text renders the bundle for a human reader.
func (b learnLessonBundle) text() string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s — %s\n", b.Concept, b.Title)
	fmt.Fprintf(&out, "  difficulty %s · %d min · %d item(s) · %d objective(s)\n", b.Difficulty, b.Minutes, len(b.Items), len(b.Objectives))
	if len(b.Prerequisites) > 0 {
		fmt.Fprintf(&out, "  prerequisites: %s\n", strings.Join(b.Prerequisites, ", "))
	}
	for i, o := range b.Objectives {
		fmt.Fprintf(&out, "  %d. %s\n", i+1, o)
	}
	if b.Example.SourceSpan != "" {
		fmt.Fprintf(&out, "  example span: %s\n", b.Example.SourceSpan)
	}
	for _, c := range b.Claims {
		fmt.Fprintf(&out, "  claim %s %s\n", c.ID, c.Digest)
	}
	for _, it := range b.Items {
		fmt.Fprintf(&out, "  [%s] %s (%s, objective %d, grading %s, provenance %s, frozen %s)\n",
			it.Type, it.Prompt, it.ID, it.Objective, it.Grading, it.Provenance, it.Frozen)
		fmt.Fprintf(&out, "        expected: %s\n", it.Expected)
	}
	for _, m := range b.Missing {
		fmt.Fprintf(&out, "  missing: %s\n", m)
	}
	return out.String()
}

// learnLessonItemByID reads one item from a concept's bank, with its effective
// freeze date. It is the lookup the grader and the recorder share, so neither can
// grade an item the bank does not carry.
func learnLessonItemByID(conceptID, itemID string) (learnLessonItem, bool) {
	return learnAssessmentItemByID(conceptID, itemID)
}

// learnLessonItemsOf reads the item rows out of the Assess section. Parsing is
// header-keyed: a column is read by name, so the three optional trailing columns
// may be absent and every absent value takes its default. A table whose header
// lacks a base column is not a lesson table and yields no items.
func learnLessonItemsOf(assess string) []learnLessonItem {
	rows := markdownTableRows(assess)
	if len(rows) < 2 {
		return nil
	}
	index := learnLessonHeaderIndex(rows[0])
	if index == nil {
		return nil
	}
	items := make([]learnLessonItem, 0, len(rows)-1)
	for _, cells := range rows[1:] {
		n := 0
		if _, err := fmt.Sscanf(learnLessonCell(cells, index, "objective"), "%d", &n); err != nil {
			n = 0
		}
		it := learnLessonItem{
			ID:         learnLessonCell(cells, index, "id"),
			Objective:  n,
			Type:       learnLessonCell(cells, index, "type"),
			Prompt:     learnLessonCell(cells, index, "prompt"),
			Expected:   learnLessonCell(cells, index, "expected"),
			Rubric:     learnLessonCell(cells, index, "rubric"),
			Distractor: learnLessonCell(cells, index, "distractor"),
			Claim:      learnLessonCell(cells, index, ctxFrontmatterClaim),
			Grading:    learnLessonDefault(learnLessonCell(cells, index, "grading"), learnGradingAuto),
			Provenance: learnLessonDefault(learnLessonCell(cells, index, "provenance"), learnProvenanceUnknown),
			Frozen:     learnLessonCell(cells, index, "frozen"),
			Hash:       learnLessonCell(cells, index, "hash"),
			KeyHash:    learnLessonCell(cells, index, "key_hash"),
		}
		it.KeyDigest = learn.KeyDigest(it.Expected, it.Rubric, it.Grading, it.Distractor)
		items = append(items, it)
	}
	return items
}

// learnLessonHeaderIndex maps every header cell to its column. It returns nil when
// a base column is missing: such a table cannot be read as a lesson.
func learnLessonHeaderIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, cell := range header {
		index[strings.ToLower(strings.TrimSpace(cell))] = i
	}
	for _, column := range ctxLessonItemColumns {
		if _, ok := index[column]; !ok {
			return nil
		}
	}
	return index
}

// learnLessonCell returns one named cell, or "" when the column is absent or the
// row is shorter than its header.
func learnLessonCell(cells []string, index map[string]int, column string) string {
	i, ok := index[column]
	if !ok || i < 0 || i >= len(cells) {
		return ""
	}
	return strings.TrimSpace(cells[i])
}

// learnLessonDefault returns a value or its default when empty.
func learnLessonDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

// learnEffectiveFrozen is the date an item's key was frozen: the item's own cell,
// else the `created` of the document that carries it (the assessment). A bank
// without either has no freeze date and says so with an empty string.
func learnEffectiveFrozen(itemFrozen, docCreated string) string {
	if f := strings.TrimSpace(itemFrozen); f != "" {
		return f
	}
	created := strings.TrimSpace(docCreated)
	if len(created) > len(learn.DateLayout) {
		return created[:len(learn.DateLayout)]
	}
	return created
}

// learnClaimCounts is the per-run tally of claim references: how many items
// resolve to a live claim, how many pin a digest that no longer matches, how
// many never pinned one, how many point at an id the page does not carry, and
// how many still cite the positional anchor.
type learnClaimCounts struct {
	OK         int
	Stale      int
	Unverified int
	Broken     int
	Legacy     int
}

// line renders the tally for the validate report. It is a report, not a finding:
// an `unverified` item is not an error, it is an item nobody re-read.
func (c learnClaimCounts) line() string {
	return fmt.Sprintf("claims: ok=%d stale=%d unverified=%d broken=%d legacy=%d", c.OK, c.Stale, c.Unverified, c.Broken, c.Legacy)
}

// learnResolveItemClaim resolves one item's claim reference: the stable id first,
// the positional anchor only as the legacy form the migration has not touched.
func learnResolveItemClaim(path, concept string, it learnLessonItem, byID map[string]contextwiki.Claim, anchorSet map[int]bool) ([]string, learnClaimCounts) {
	var issues []string
	var counts learnClaimCounts
	pageID, ref, ok := strings.Cut(it.Claim, "#")
	if !ok || pageID != concept {
		return []string{fmt.Sprintf("%s: item %q cites %q, want %q", path, it.ID, it.Claim, concept+"#c-<id>")}, counts
	}
	switch {
	case strings.HasPrefix(ref, "c-"):
		claim, live := byID[ref]
		if !live {
			counts.Broken++
			return []string{fmt.Sprintf("%s: item %q cites claim %s, which page %q does not carry (broken)", path, it.ID, ref, concept)}, counts
		}
		switch it.Hash {
		case "":
			counts.Unverified++
			issues = append(issues, fmt.Sprintf("%s: item %q pins no claim digest (the `hash` cell); a new item must record the claim it was written against", path, it.ID))
		case claim.Digest:
			counts.OK++
		default:
			counts.Stale++
			issues = append(issues, fmt.Sprintf("%s: item %q is stale: claim %s now reads a different statement (item hash %s, page %s)", path, it.ID, ref, it.Hash, claim.Digest))
		}
	case strings.HasPrefix(ref, "claim-"):
		number, convErr := strconv.Atoi(strings.TrimPrefix(ref, "claim-"))
		if convErr != nil || !anchorSet[number] {
			return []string{fmt.Sprintf("%s: item %q cites claim %s, which page %q does not carry", path, it.ID, ref, concept)}, counts
		}
		counts.Legacy++
		issues = append(issues, fmt.Sprintf("%s: item %q still cites the positional anchor %q — migrate it to the stable id", path, it.ID, it.Claim))
	default:
		issues = append(issues, fmt.Sprintf("%s: item %q cites %q, want %q", path, it.ID, it.Claim, concept+"#c-<id>"))
	}
	return issues, counts
}

// learnValidateAll is the cross-file check: for every lesson under
// context/learning, the shape is linted and the references are resolved against
// the page it teaches. The counts are returned beside the findings.
func learnValidateAll(bld *contextwiki.Builder) ([]string, learnClaimCounts) {
	files, err := walkMarkdownFiles(sdtLearningDir)
	if err != nil {
		return []string{fmt.Sprintf("%s: %s", sdtLearningDir, err)}, learnClaimCounts{}
	}
	sort.Strings(files)
	var issues []string
	var counts learnClaimCounts
	for _, path := range files {
		if learnIsAssessmentPath(path) {
			found, c := learnValidateAssessment(bld, path)
			issues = append(issues, found...)
			counts.OK += c.OK
			counts.Stale += c.Stale
			counts.Unverified += c.Unverified
			counts.Broken += c.Broken
			counts.Legacy += c.Legacy
			continue
		}
		found, c := learnValidateLesson(bld, path)
		issues = append(issues, found...)
		counts.OK += c.OK
		counts.Stale += c.Stale
		counts.Unverified += c.Unverified
		counts.Broken += c.Broken
		counts.Legacy += c.Legacy
	}
	return issues, counts
}

// learnValidateLesson checks one lesson document against its page.
func learnValidateLesson(bld *contextwiki.Builder, path string) ([]string, learnClaimCounts) {
	var counts learnClaimCounts
	data, err := os.ReadFile(path) //#nosec G304 -- path from the corpus walk
	if err != nil {
		return []string{fmt.Sprintf("%s: %s", path, err)}, counts
	}
	content := string(data)
	var issues []string
	for _, i := range lintLesson(path, content, ctxTypeLesson) {
		issues = append(issues, fmt.Sprintf("%s: %s", path, i.Message))
	}
	// The lesson's id is its path under context/learning; that id is the concept.
	rel, err := filepath.Rel(sdtLearningDir, path)
	if err != nil {
		return append(issues, fmt.Sprintf("%s: %s", path, err)), counts
	}
	concept := strings.TrimSuffix(filepath.ToSlash(rel), sdtMarkdownExt)
	page := bld.Resolve(concept)
	if page == nil {
		return append(issues, fmt.Sprintf("%s: no wiki page %q for this lesson", path, concept)), counts
	}
	objectives := learningObjectivesOf(page.Frontmatter)
	if len(objectives) == 0 {
		issues = append(issues, fmt.Sprintf("%s: page %q carries no `learning.objectives` to align items with", path, concept))
	}
	// The items live in the concept's assessment document: the lesson no longer
	// carries a table of its own, so a concept without a bank has no item at all.
	items, hasBank := learnAssessmentItems(concept)
	if !hasBank {
		issues = append(issues, fmt.Sprintf("%s: no assessment document at %s (the item bank of %q)", path, learnAssessmentPath(concept), concept))
		for i := 1; i <= len(objectives); i++ {
			issues = append(issues, fmt.Sprintf("%s: objective %d has no item", path, i))
		}
		return issues, counts
	}
	found, c := learnValidateItems(bld, path, concept, items, objectives, parseFrontmatterField(content, "updated"))
	issues = append(issues, found...)
	counts.OK += c.OK
	counts.Stale += c.Stale
	counts.Unverified += c.Unverified
	counts.Broken += c.Broken
	counts.Legacy += c.Legacy
	return issues, counts
}

// learnValidateItems checks one concept's item bank against the page it assesses:
// the vocabulary of every cell, the objective it points at, and the claim digest
// it was written against. It is called from the lesson pass (through the
// concept's bank) and from the assessment pass (a bank with no lesson), so one
// document's items are checked once, by the same rules.
func learnValidateItems(bld *contextwiki.Builder, path, concept string, items []learnLessonItem, objectives []string, updated string) ([]string, learnClaimCounts) {
	var issues []string
	var counts learnClaimCounts
	page := bld.Resolve(concept)
	if page == nil {
		return append(issues, fmt.Sprintf("%s: no wiki page %q for this item bank", path, concept)), counts
	}
	// Claims are resolved by the stable id; the positional number is accepted
	// only as a legacy reference, and reported so it gets migrated.
	byID := map[string]contextwiki.Claim{}
	anchorSet := map[int]bool{}
	for _, c := range page.Claims() {
		if c.ID != "" {
			byID[c.ID] = c
		}
		anchorSet[c.Number] = true
	}
	covered := map[int]bool{}
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it.ID] {
			issues = append(issues, fmt.Sprintf("%s: item id %q appears twice", path, it.ID))
		}
		seen[it.ID] = true
		if !containsItem(learnLessonGradingRules, it.Grading) {
			issues = append(issues, fmt.Sprintf("%s: item %q grading %q is not one of %s",
				path, it.ID, it.Grading, strings.Join(learnLessonGradingRules, ", ")))
		}
		if !containsItem(learnLessonProvenances, it.Provenance) {
			issues = append(issues, fmt.Sprintf("%s: item %q provenance %q is not one of %s",
				path, it.ID, it.Provenance, strings.Join(learnLessonProvenances, ", ")))
		}
		if it.Frozen != "" && updated != "" && it.Frozen > updated[:min(len(updated), len(learn.DateLayout))] {
			issues = append(issues, fmt.Sprintf("%s: item %q is frozen at %s, after the document was last updated (%s)",
				path, it.ID, it.Frozen, updated))
		}
		if it.Objective < 1 || it.Objective > len(objectives) {
			issues = append(issues, fmt.Sprintf("%s: item %q points at objective %d, the page has %d", path, it.ID, it.Objective, len(objectives)))
		} else {
			covered[it.Objective] = true
		}
		found, c := learnResolveItemClaim(path, concept, it, byID, anchorSet)
		issues = append(issues, found...)
		counts.OK += c.OK
		counts.Stale += c.Stale
		counts.Unverified += c.Unverified
		counts.Broken += c.Broken
		counts.Legacy += c.Legacy
	}
	for i := 1; i <= len(objectives); i++ {
		if !covered[i] {
			issues = append(issues, fmt.Sprintf("%s: objective %d has no item", path, i))
		}
	}
	return issues, counts
}

// learnValidateAssessment checks one assessment document: its items against the
// page, and the concept it names in `concept:`. A bank with a lesson is checked
// through its lesson; one without has no lesson pass to run, so this is its only
// check.
func learnValidateAssessment(bld *contextwiki.Builder, path string) ([]string, learnClaimCounts) {
	var counts learnClaimCounts
	data, err := os.ReadFile(path) //#nosec G304 -- path from the corpus walk
	if err != nil {
		return []string{fmt.Sprintf("%s: %s", path, err)}, counts
	}
	content := string(data)
	concept, ok := learnConceptOfAssessmentPath(path)
	if !ok {
		return []string{fmt.Sprintf("%s: not a document under %s", path, sdtLearningDir)}, counts
	}
	var issues []string
	for _, i := range lintAssessment(path, content, ctxTypeAssessment) {
		issues = append(issues, fmt.Sprintf("%s: %s", path, i.Message))
	}
	if named := parseFrontmatterField(content, ctxFrontmatterConcept); named != concept {
		return append(issues, fmt.Sprintf("%s: concept %q does not name the concept its path asserts (%q)", path, named, concept)), counts
	}
	page := bld.Resolve(concept)
	if page == nil {
		return append(issues, fmt.Sprintf("%s: no wiki page %q for this item bank", path, concept)), counts
	}
	if learnLessonExists(concept) {
		// The lesson pass reads this bank and reports through the lesson's path; a
		// second report would be the same finding twice.
		return issues, counts
	}
	found, c := learnValidateItems(bld, path, concept, learnAssessmentItemsOf(content), learningObjectivesOf(page.Frontmatter), parseFrontmatterField(content, "updated"))
	return append(issues, found...), c
}

// sectionOf returns one `##` section of a document body, frontmatter stripped.
func sectionOf(content, name string) string {
	_, sections := markdownSections(documentBody(content))
	return sections[name]
}

// learnCoverage builds the objective-to-item matrix for every concept that
// carries a lesson or a bank. The two share a directory, so one walk feeds both:
// a concept appears once, whichever document it was reached through.
func learnCoverage(bld *contextwiki.Builder) []learnCoverageRow {
	files, err := walkMarkdownFiles(sdtLearningDir)
	if err != nil {
		return nil
	}
	sort.Strings(files)
	rows := make([]learnCoverageRow, 0, len(files))
	seen := map[string]bool{}
	for _, path := range files {
		if learnIsLevelPath(path) {
			continue
		}
		concept := ""
		if c, ok := learnConceptOfAssessmentPath(path); ok {
			concept = c
		} else {
			rel, err := filepath.Rel(sdtLearningDir, path)
			if err != nil {
				continue
			}
			concept = strings.TrimSuffix(filepath.ToSlash(rel), sdtMarkdownExt)
		}
		if seen[concept] {
			continue
		}
		seen[concept] = true
		row := learnCoverageRow{Concept: concept}
		page := bld.Resolve(concept)
		if page != nil {
			row.Objectives = len(learningObjectivesOf(page.Frontmatter))
		}
		items := learnAssessmentObjectiveRefs(concept)
		row.Items = len(items)
		row.Uncovered = learnUncoveredObjectives(row.Objectives, items)
		rows = append(rows, row)
	}
	return rows
}

func init() {
	learnLessonCmd.Flags().Bool("validate", false, "Validate every lesson against its page (read-only)")
	learnLessonCmd.Flags().Bool("coverage", false, "Print the objective to item matrix (read-only)")
	learnCmd.AddCommand(learnLessonCmd)
}
