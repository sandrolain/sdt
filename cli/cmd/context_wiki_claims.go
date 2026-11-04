package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/distill"
)

// ── claim span/atomicity signal (sdt context wiki claims) ────────────────────
//
// A claim that cites forty lines or three units is a paraphrase, not an atomic
// assertion: the reader cannot tell which sentence rests on which passage. The
// check is a SUGGESTION so a legitimate multi-step claim stays visible without
// blocking the corpus.

type claimFinding struct {
	Page       string `json:"page" yaml:"page"`
	Line       int    `json:"line" yaml:"line"`
	Claim      string `json:"claim" yaml:"claim"`
	Ref        string `json:"ref" yaml:"ref"`
	Start      int    `json:"start" yaml:"start"`
	End        int    `json:"end" yaml:"end"`
	Lines      int    `json:"lines" yaml:"lines"`
	Units      int    `json:"units" yaml:"units"`
	Unresolved bool   `json:"unresolved,omitempty" yaml:"unresolved,omitempty"`
}

type claimBlock struct {
	ID   string
	Line int
	Text string
}

var contextWikiClaimsCmd = &cobra.Command{
	Use:   "claims [page...]",
	Short: "Flag broad claims that cite too many lines or units",
	Long: `Flag claims whose citation is too broad to be atomic.

A claim block starts at its {#claim-N} anchor and runs to the next anchor. For
each citation in the block the command counts the cited artifact lines and the
artifact units the span overlaps. A citation over --max-lines (default 40) or
over --max-units (default 2) is a SUGGESTION: the command names the page, claim
id, citation, line count and unit count, exits 0 and never edits a page.

With --validate the same walk selects the validation shortlist instead: one row
per target, where a target is a page whose claims all rest on one source (or
whose maturity or validity says they age) or a single claim whose own prose
carries "inferred" or "not stated in the source". The report only selects; a
person decides (ingestion.md, *The validation lane*).

Examples:
  sdt context wiki claims
  sdt context wiki claims architecture/deployment-pipeline --max-lines 80
  sdt context wiki claims --format json
  sdt context wiki claims --validate
  sdt context wiki claims --validate --horizon 30 --format json`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if getBoolFlag(cmd, "budget", false) {
			runClaimBudget(cmd, args)
			return
		}
		if cmd.Flags().Changed("at") {
			exitWithError(cmd, fmt.Errorf("--at lowers the budget threshold and is only valid with --budget"))
			return
		}
		if getBoolFlag(cmd, "validate", false) {
			if getBoolFlag(cmd, "budget", false) {
				exitWithError(cmd, fmt.Errorf("--validate and --budget are different reports; ask for one"))
				return
			}
			runClaimValidate(cmd, args)
			return
		}
		paths, err := claimPages(args)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		maxLines := getIntFlag(cmd, "max-lines", false)
		maxUnits := getIntFlag(cmd, "max-units", false)
		if maxLines < 1 || maxUnits < 1 {
			exitWithError(cmd, fmt.Errorf("--max-lines and --max-units must be positive"))
			return
		}
		var findings []claimFinding
		cache := map[string]artifactSpan{}
		for _, path := range paths {
			pageFindings, ferr := claimFindings(path, maxLines, maxUnits, cache)
			if ferr != nil {
				exitWithError(cmd, ferr)
				return
			}
			findings = append(findings, pageFindings...)
		}
		outputClaimFindings(cmd, findings)
	},
}

func claimPages(args []string) ([]string, error) {
	if len(args) > 0 {
		out := make([]string, 0, len(args))
		for _, ref := range args {
			path, err := resolveWikiPage(ref)
			if err != nil {
				return nil, err
			}
			out = append(out, path)
		}
		return out, nil
	}
	bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
	if err != nil {
		return nil, fmt.Errorf("load wiki: %w", err)
	}
	out := make([]string, 0, len(bld.Pages))
	for _, page := range bld.Pages {
		out = append(out, page.Path)
	}
	return out, nil
}

type artifactSpan struct {
	units []distill.Unit
	lines int
}

func claimFindings(path string, maxLines, maxUnits int, cache map[string]artifactSpan) ([]claimFinding, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus path resolved under context/wiki/
	if err != nil {
		return nil, err
	}
	content := string(data)
	fm, body := contextwiki.SplitFrontmatter(content)
	offset := strings.Count(fm, "\n")
	var findings []claimFinding
	for _, block := range claimBlocks(body) {
		for _, cite := range distill.ParseCitations(block.Text) {
			artifact, rerr := resolveCorpusFile(cite.Ref)
			if rerr != nil {
				findings = append(findings, claimFinding{
					Page: corpusRelative(path), Line: offset + block.Line, Claim: block.ID,
					Ref: cite.Ref, Start: cite.Line, End: cite.End, Unresolved: true,
				})
				continue
			}
			span, ok := cache[artifact]
			if !ok {
				artData, aerr := os.ReadFile(artifact) //#nosec G304 -- corpus path resolved under context/
				if aerr != nil {
					return nil, aerr
				}
				artUnits, _ := distill.Parse(string(artData), distill.Options{})
				span = artifactSpan{units: artUnits, lines: distill.BodyLines(string(artData))}
				cache[artifact] = span
			}
			lines, covered := claimSpan(cite, span)
			if lines > maxLines || covered > maxUnits {
				findings = append(findings, claimFinding{
					Page:  corpusRelative(path),
					Line:  offset + block.Line,
					Claim: block.ID,
					Ref:   cite.Ref,
					Start: cite.Line,
					End:   cite.End,
					Lines: lines,
					Units: covered,
				})
			}
		}
	}
	return findings, nil
}

// claimBlocks splits a body at {#claim-N} anchors. Prose before the first
// anchor belongs to no claim and is ignored.
func claimBlocks(body string) []claimBlock {
	lines := strings.Split(body, "\n")
	var blocks []claimBlock
	current := -1
	for i, line := range lines {
		anchor := contextwiki.ClaimRegexp.FindString(line)
		if anchor == "" {
			if current >= 0 {
				blocks[current].Text += "\n" + line
			}
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(anchor, "{#"), "}")
		blocks = append(blocks, claimBlock{ID: id, Line: i + 1, Text: line})
		current = len(blocks) - 1
	}
	return blocks
}

func claimSpan(cite distill.Citation, span artifactSpan) (lines, covered int) {
	if cite.Whole {
		for _, u := range span.units {
			if u.Kind != distill.UnitContainer {
				covered++
			}
		}
		return span.lines, covered
	}
	if cite.End < cite.Line {
		return 0, 0
	}
	lines = cite.End - cite.Line + 1
	for _, u := range span.units {
		if u.Kind == distill.UnitContainer {
			continue
		}
		if cite.Line <= u.End && cite.End >= u.Line {
			covered++
		}
	}
	return lines, covered
}

// claimBudgetFinding is one page over the claim budget.
type claimBudgetFinding struct {
	Page   string `json:"page" yaml:"page"`
	Claims int    `json:"claims" yaml:"claims"`
	Budget int    `json:"budget" yaml:"budget"`
	Over   int    `json:"over" yaml:"over"`
}

// runClaimBudget reports pages over the claim budget. The default threshold is
// the lint's own constant, so the report and `wiki lint` cannot disagree.
func runClaimBudget(cmd *cobra.Command, args []string) {
	threshold := contextwiki.BudgetClaims
	if cmd.Flags().Changed("at") {
		threshold = getIntFlag(cmd, "at", false)
		if threshold < 1 {
			exitWithError(cmd, fmt.Errorf("--at must be positive"))
			return
		}
	}
	pages, err := claimBudgetPages(args)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	outputClaimBudget(cmd, budgetFindings(pages, threshold))
}

// claimBudgetPages resolves the pages the budget report reads.
func claimBudgetPages(args []string) ([]*contextwiki.Page, error) {
	if len(args) == 0 {
		bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
		if err != nil {
			return nil, fmt.Errorf("load wiki: %w", err)
		}
		return bld.Pages, nil
	}
	out := make([]*contextwiki.Page, 0, len(args))
	for _, ref := range args {
		path, err := resolveWikiPage(ref)
		if err != nil {
			return nil, err
		}
		page, err := contextwiki.ParsePage(sdtWikiDir, path)
		if err != nil {
			return nil, err
		}
		out = append(out, page)
	}
	return out, nil
}

// budgetFindings keeps the pages over the threshold, worst first.
func budgetFindings(pages []*contextwiki.Page, threshold int) []claimBudgetFinding {
	var out []claimBudgetFinding
	for _, p := range pages {
		if p.ClaimCount <= threshold {
			continue
		}
		out = append(out, claimBudgetFinding{
			Page:   p.Path,
			Claims: p.ClaimCount,
			Budget: threshold,
			Over:   p.ClaimCount - threshold,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Over != out[j].Over {
			return out[i].Over > out[j].Over
		}
		return out[i].Page < out[j].Page
	})
	return out
}

func outputClaimBudget(cmd *cobra.Command, findings []claimBudgetFinding) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(findings, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(findings)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		for _, f := range findings {
			outputString(cmd, fmt.Sprintf("[WARNING] %s: page has %d claims (budget ~%d): over budget by %d\n", f.Page, f.Claims, f.Budget, f.Over))
		}
	}
}

// ── validation shortlist (sdt context wiki claims --validate) ────────────────
//
// The shortlist names what deserves a second source: the pages whose claims rest
// on one source only, the claims that age on their own, and the claims whose own
// prose says the evidence is inferred or absent. The report only selects; a
// person decides, and the ingestion run records the decision (ingestion.md,
// *The validation lane*).

// validateRow is one shortlist target: a page whose claims all rest on the same
// footing, or a single claim that carries its own marker.
type validateRow struct {
	Page       string   `json:"page" yaml:"page"`
	Line       int      `json:"line" yaml:"line"`
	Target     string   `json:"target" yaml:"target"` // page | claim
	Claim      string   `json:"claim,omitempty" yaml:"claim,omitempty"`
	Claims     int      `json:"claims,omitempty" yaml:"claims,omitempty"`
	Signals    []string `json:"signals" yaml:"signals"`
	Reason     string   `json:"reason" yaml:"reason"`
	Confidence string   `json:"confidence,omitempty" yaml:"confidence,omitempty"`
	Maturity   string   `json:"maturity,omitempty" yaml:"maturity,omitempty"`
	Validity   string   `json:"validity,omitempty" yaml:"validity,omitempty"`
	ValidUntil string   `json:"valid_until,omitempty" yaml:"valid_until,omitempty"`
}

// The two prose markers a distilled claim already carries when its author could
// not vouch for it. They are matched on the claim block, case-insensitively.
var validateClaimMarkers = []string{"inferred", "not stated in the source"}

// The shortlist's two target kinds: a page whose claims all rest on one source,
// or a single claim. They are the JSON `target` values and the text label.
const (
	validateTargetPage  = "page"
	validateTargetClaim = "claim"
)

func runClaimValidate(cmd *cobra.Command, args []string) {
	horizon := getIntFlag(cmd, "horizon", false)
	if horizon < 0 {
		exitWithError(cmd, fmt.Errorf("--horizon must not be negative"))
		return
	}
	paths, err := claimPages(args)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	var rows []validateRow
	for _, path := range paths {
		pageRows, rerr := validateRows(path, horizon, time.Now())
		if rerr != nil {
			exitWithError(cmd, rerr)
			return
		}
		rows = append(rows, pageRows...)
	}
	sortValidateRows(rows)
	outputValidateRows(cmd, rows)
}

// validateRows selects the targets of one page. A page-level signal produces one
// row for the page and names any claim-level marker inside its reason; only a
// page with no page-level signal emits per-claim rows, so a target is never
// listed twice.
func validateRows(path string, horizonDays int, now time.Time) ([]validateRow, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus path resolved under context/wiki/
	if err != nil {
		return nil, err
	}
	content := string(data)
	fm, body := contextwiki.SplitFrontmatter(content)
	offset := strings.Count(fm, "\n")
	values := contextwiki.FrontmatterValues(content)
	field := func(key string) string {
		if v := values[key]; len(v) > 0 {
			return strings.TrimSpace(v[0])
		}
		return ""
	}
	confidence := strings.ToLower(field("confidence"))
	maturity := strings.ToLower(field("maturity"))
	validity := strings.ToLower(field("validity"))
	validUntil := field("valid_until")
	page := corpusRelative(path)
	blocks := claimBlocks(body)
	marked := markedClaims(blocks)

	row := validateRow{
		Page: page, Line: offset + 1, Target: validateTargetPage, Claims: len(blocks),
		Confidence: confidence, Maturity: maturity, Validity: validity, ValidUntil: validUntil,
	}
	if confidence == "low" || confidence == "medium" {
		row.Signals = append(row.Signals, "confidence")
		row.Reason = appendReason(row.Reason, fmt.Sprintf("confidence %s: one source only", confidence))
	}
	if validity == "evolving" {
		due, known := validUntilDue(validUntil, horizonDays, now)
		if due {
			row.Signals = append(row.Signals, "validity")
			if known {
				row.Reason = appendReason(row.Reason, fmt.Sprintf("validity evolving, valid_until %s inside %dd", validUntil, horizonDays))
			} else {
				row.Reason = appendReason(row.Reason, "validity evolving with no valid_until")
			}
		}
	}
	if maturity == ctxMaturityWip || maturity == ctxMaturityProvisional {
		row.Signals = append(row.Signals, "maturity")
		row.Reason = appendReason(row.Reason, fmt.Sprintf("maturity %s", maturity))
	}
	if len(row.Signals) > 0 {
		if len(marked) > 0 {
			row.Reason = appendReason(row.Reason, "claims marked: "+strings.Join(marked, ", "))
		}
		return []validateRow{row}, nil
	}

	rows := make([]validateRow, 0, len(marked))
	for _, block := range blocks {
		marker := claimMarker(block.Text)
		if marker == "" {
			continue
		}
		rows = append(rows, validateRow{
			Page: page, Line: offset + block.Line, Target: validateTargetClaim, Claim: block.ID,
			Signals: []string{"inferred"}, Reason: "claim marked " + marker,
			Confidence: confidence, Maturity: maturity, Validity: validity, ValidUntil: validUntil,
		})
	}
	return rows, nil
}

// markedClaims lists the claim ids that carry a prose marker, in body order.
func markedClaims(blocks []claimBlock) []string {
	var out []string
	for _, block := range blocks {
		if claimMarker(block.Text) != "" {
			out = append(out, "#"+block.ID)
		}
	}
	return out
}

// claimMarker returns the first marker the claim block carries, lower-cased, or
// "" when the claim does not hedge its own evidence.
func claimMarker(text string) string {
	lower := strings.ToLower(text)
	for _, marker := range validateClaimMarkers {
		if strings.Contains(lower, marker) {
			return marker
		}
	}
	return ""
}

// validUntilDue reports whether a claim that ages on its own is inside the
// horizon. A missing or unparseable date is due: an undated evolving claim has
// no evidence it is still current.
func validUntilDue(validUntil string, horizonDays int, now time.Time) (due, known bool) {
	raw := strings.TrimSpace(validUntil)
	if raw == "" {
		return true, false
	}
	var parsed time.Time
	var err error
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01"} {
		if parsed, err = time.Parse(layout, raw); err == nil {
			break
		}
	}
	if err != nil {
		return true, false
	}
	return !parsed.After(now.AddDate(0, 0, horizonDays)), true
}

func appendReason(current, addition string) string {
	if current == "" {
		return addition
	}
	return current + "; " + addition
}

func sortValidateRows(rows []validateRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Page != rows[j].Page {
			return rows[i].Page < rows[j].Page
		}
		return rows[i].Line < rows[j].Line
	})
}

func outputValidateRows(cmd *cobra.Command, rows []validateRow) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(rows, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(rows)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		for _, r := range rows {
			target := validateTargetPage
			if r.Target == validateTargetClaim {
				target = "#" + r.Claim
			}
			outputString(cmd, fmt.Sprintf("[VALIDATE] %s:%d %s claims=%d signals=%s reason=%s\n",
				r.Page, r.Line, target, r.Claims, strings.Join(r.Signals, ","), r.Reason))
		}
	}
}

func outputClaimFindings(cmd *cobra.Command, findings []claimFinding) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(findings, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(findings)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		for _, f := range findings {
			if f.Unresolved {
				outputString(cmd, fmt.Sprintf("[SUGGESTION] %s:%d %s: %s unresolved\n", f.Page, f.Line, f.Claim, f.Ref))
				continue
			}
			outputString(cmd, fmt.Sprintf("[SUGGESTION] %s:%d %s: %s:%d-%d lines=%d units=%d\n",
				f.Page, f.Line, f.Claim, f.Ref, f.Start, f.End, f.Lines, f.Units))
		}
	}
}

func init() {
	contextWikiClaimsCmd.Flags().Int("max-lines", 40, "Flag a citation longer than this many artifact lines")
	contextWikiClaimsCmd.Flags().Int("max-units", 2, "Flag a citation overlapping more than this many artifact units")
	contextWikiClaimsCmd.Flags().Bool("budget", false, "Report pages over the claim budget, worst first")
	contextWikiClaimsCmd.Flags().Bool("validate", false, "Report the validation shortlist: pages and claims that deserve a second source")
	contextWikiClaimsCmd.Flags().Int("horizon", 180, "With --validate, days ahead that make an evolving claim due")
	contextWikiClaimsCmd.Flags().Int("at", 0, "With --budget, lower the threshold (default: the lint's constant)")
	contextWikiCmd.AddCommand(contextWikiClaimsCmd)
}
