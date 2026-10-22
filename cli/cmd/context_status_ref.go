package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/spf13/cobra"
)

// ── context document resolution (status get/set, rename) ───────────────────────

// ctxResolvedDoc is an existing context/ work file with its registry type.

type ctxResolvedDoc struct {
	Path string
	Type ctxDocType
}

// resolveContextDoc accepts a path under context/ (absolute or relative, with
// or without the .md suffix) or identity flags (--type/--slug plus
// --number/--phase/--plan) and returns the existing file plus its type.

func resolveContextDoc(cmd *cobra.Command, args []string) (ctxResolvedDoc, error) {
	if len(args) > 0 {
		return resolveContextDocPath(args[0])
	}
	return resolveContextDocIdentity(cmd)
}

// ctxCwd returns the current working directory used as the base for resolving
// absolute document paths.
func ctxCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// resolveContextDocPath resolves a reference to an existing context document.
// The steps are tried in order, and a step runs only when the previous one
// found nothing:
//
//  1. an explicit path — absolute, context/-prefixed, or kind/name;
//  2. a bare file name across the registered kind directories;
//  3. a wiki page by its frontmatter id, or by its file name at any depth under
//     context/wiki;
//  4. a uid, when the reference is a canonical UUIDv7.
//
// An ambiguous reference is refused with its candidates rather than guessed.
// When no step matches, the path error is returned unchanged, so a typo keeps
// its familiar message.
func resolveContextDocPath(ref string) (ctxResolvedDoc, error) {
	doc, pathErr := resolveContextDocFilePath(ref)
	if pathErr == nil {
		return doc, nil
	}
	// An ambiguity is a conclusive answer: the caller named a document twice,
	// and an id must not silently pick one of them.
	var ambiguous *ctxAmbiguousRefError
	if errors.As(pathErr, &ambiguous) {
		return ctxResolvedDoc{}, pathErr
	}
	if !ctxRefMayNameADocument(ref) {
		return ctxResolvedDoc{}, pathErr
	}
	doc, err := resolveContextDocByRefIdentity(ref)
	if err == nil {
		return doc, nil
	}
	if !errors.Is(err, errNoContextRefMatch) {
		return ctxResolvedDoc{}, err
	}
	return ctxResolvedDoc{}, pathErr
}

// resolveContextDocFilePath resolves the path-shaped references: an explicit
// path, or a bare file name in a registered kind directory.
func resolveContextDocFilePath(ref string) (ctxResolvedDoc, error) {
	ref = strings.TrimSpace(ref)
	// A full markdown path (including a suffix-modified one such as
	// `.slide.md` or `.map.md`) keeps its extension: only a bare ref has
	// ".md" appended later. Trimming a blind ".md" here would turn
	// "x.slide.md" into "x.slide" and then re-append ".md" onto the wrong
	// stem.
	hadExt := strings.HasSuffix(ref, sdtMarkdownExt)
	if hadExt {
		ref = strings.TrimSuffix(ref, sdtMarkdownExt)
	}
	if ref == "" {
		return ctxResolvedDoc{}, errors.New("document reference is required (a path or --type/--slug identity)")
	}

	var candidate string
	switch {
	case filepath.IsAbs(ref):
		rel, err := filepath.Rel(ctxCwd(), ref)
		if err != nil {
			return ctxResolvedDoc{}, err
		}
		candidate = filepath.Clean(rel)
	case strings.HasPrefix(ref, sdtWorkDir+"/"):
		candidate = filepath.Clean(ref)
	default:
		// A bare file name (`20261020-130531-x.md`) is a name, not a path: look
		// for it in the directory of every registered kind, and refuse an
		// ambiguous one instead of guessing. Only then fall back to the old
		// "context/<ref>" shape, which produces the honest "unknown type" error.
		if !strings.ContainsAny(ref, `/\\`) {
			if doc, err := resolveBareContextDoc(ref, hadExt); err == nil {
				return doc, nil
			} else if !errors.Is(err, errNoBareContextMatch) {
				return ctxResolvedDoc{}, err
			}
		}
		// accept "plan/foo" as a shorthand for "context/plan/foo"
		candidate = filepath.Clean(filepath.Join(sdtWorkDir, ref))
	}

	if candidate == sdtWorkDir || !strings.HasPrefix(candidate, sdtWorkDir+string(filepath.Separator)) {
		return ctxResolvedDoc{}, fmt.Errorf("path %q is outside %s/", ref, sdtWorkDir)
	}

	// A full markdown ref had its ".md" stripped above: restore it exactly.
	// Otherwise tmp entries have no suffix; every other bare stem is markdown.
	switch {
	case hadExt:
		candidate += sdtMarkdownExt
	case filepath.Ext(candidate) == "" && ctxFirstSegment(candidate) != ctxTypeTmp:
		candidate += sdtMarkdownExt
	}

	t, ok := ctxTypeForDir(ctxFirstSegmentDir(candidate))
	if !ok {
		return ctxResolvedDoc{}, fmt.Errorf("unknown context type for path %q", candidate)
	}

	info, err := os.Stat(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return ctxResolvedDoc{}, fmt.Errorf("no such document: %s", candidate)
		}
		return ctxResolvedDoc{}, err
	}
	if info.IsDir() {
		return ctxResolvedDoc{}, fmt.Errorf("%s is a directory", candidate)
	}
	return ctxResolvedDoc{Path: candidate, Type: t}, nil
}

// errNoBareContextMatch means a bare name matched no kind directory, so the
// caller keeps its ordinary path resolution (and its ordinary error).
var errNoBareContextMatch = errors.New("no document with that name")

// resolveBareContextDoc finds a document by file name alone across the
// registered kind directories. One match resolves; several are refused with the
// candidates, because a bare name is a convenience and never a guess.
func resolveBareContextDoc(ref string, hadExt bool) (ctxResolvedDoc, error) {
	// The caller stripped a trailing ".md" before reaching here, so the name is
	// always rebuilt with it: a bare stem and a full markdown ref both resolve.
	name := ref + sdtMarkdownExt
	var matches []string
	for _, t := range contextTypeList {
		path := filepath.Join(t.dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			matches = append(matches, path)
		}
	}
	// No exact file: a leading timestamp is an id, and a document is named
	// `<id>-<slug>.md`. Match the prefix the same way, and refuse several.
	if len(matches) == 0 {
		prefix := strings.TrimSuffix(name, sdtMarkdownExt) + "-"
		for _, t := range contextTypeList {
			entries, err := os.ReadDir(t.dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) || !strings.HasSuffix(e.Name(), sdtMarkdownExt) {
					continue
				}
				matches = append(matches, filepath.Join(t.dir, e.Name()))
			}
		}
	}
	switch len(matches) {
	case 0:
		return ctxResolvedDoc{}, errNoBareContextMatch
	case 1:
		t, _ := ctxTypeForDir(filepath.Dir(matches[0]))
		return ctxResolvedDoc{Path: matches[0], Type: t}, nil
	default:
		return ctxResolvedDoc{}, &ctxAmbiguousRefError{Ref: ref, Matches: matches}
	}
}

// ctxAmbiguousRefError refuses a reference that named several documents. It is
// a conclusive answer: the identity steps must not turn it into a guess.
type ctxAmbiguousRefError struct {
	Ref     string
	Matches []string
	Hint    string
}

func (e *ctxAmbiguousRefError) Error() string {
	hint := e.Hint
	if hint == "" {
		hint = "name the directory"
	}
	return fmt.Sprintf("%q matches more than one document (%s) — %s", e.Ref, strings.Join(e.Matches, ", "), hint)
}

// errNoContextRefMatch means neither the wiki step nor the uid step named a
// document, so the caller keeps the path error it already had.
var errNoContextRefMatch = errors.New("no document with that name or id")

// ctxRefMayNameADocument reports whether a reference is corpus-internal, so the
// identity steps may be tried on it. An absolute path, or one that climbs out
// of the tree, is a path error and never a name.
func ctxRefMayNameADocument(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" || filepath.IsAbs(ref) {
		return false
	}
	clean := filepath.Clean(ref)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

// resolveContextDocByRefIdentity resolves a reference by what a document calls
// itself: a wiki id or page name, then a uid. The wiki step runs first, so a
// page's own id wins over a uid-shaped value that happens to be stored in one.
func resolveContextDocByRefIdentity(ref string) (ctxResolvedDoc, error) {
	matches, err := ctxWikiRefMatches(ref)
	if err != nil {
		return ctxResolvedDoc{}, err
	}
	if len(matches) == 0 && validUIDv7(ref) {
		matches, err = ctxUIDRefMatches(ref)
		if err != nil {
			return ctxResolvedDoc{}, err
		}
	}
	switch len(matches) {
	case 0:
		return ctxResolvedDoc{}, errNoContextRefMatch
	case 1:
		return ctxDocForPath(matches[0])
	default:
		return ctxResolvedDoc{}, &ctxAmbiguousRefError{Ref: ref, Matches: matches, Hint: "name it by its context/ path"}
	}
}

// ctxWikiRefMatches finds wiki pages by frontmatter id, or — for a reference
// without a separator — by file name at any depth. A page matching both shapes
// is reported once. It reads through the same loader the graph uses, so a page
// that cannot be parsed cannot be named by id either.
func ctxWikiRefMatches(ref string) ([]string, error) {
	bld, _, walkErr := contextwiki.LoadWiki(sdtWikiDir)
	if walkErr != nil {
		if os.IsNotExist(walkErr) {
			return nil, nil
		}
		return nil, walkErr
	}
	bareName := !strings.ContainsAny(ref, `/\`)
	var matches []string
	for _, p := range bld.Pages {
		hit := bareName && strings.TrimSuffix(p.Base, sdtMarkdownExt) == ref
		if !hit {
			hit = p.ID == ref
		}
		if hit {
			matches = append(matches, p.Path)
		}
	}
	sort.Strings(matches)
	return matches, nil
}

// ctxUIDRefMatches finds the identifier-eligible document carrying a uid.
// Only a canonical UUIDv7 reaches this scan, so an ordinary reference pays no
// tree walk.
func ctxUIDRefMatches(uid string) ([]string, error) {
	files, err := ctxUIDFiles()
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, path := range files {
		//#nosec G304 -- identifier-eligible corpus document
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		if strings.TrimSpace(frontmatterField(string(data), ctxFrontmatterUID)) == uid {
			matches = append(matches, path)
		}
	}
	return matches, nil
}

// ctxDocForPath builds the resolved document for a path found by name or id,
// deriving its type from the first segment under context/.
func ctxDocForPath(path string) (ctxResolvedDoc, error) {
	t, ok := ctxTypeForDir(ctxFirstSegmentDir(path))
	if !ok {
		return ctxResolvedDoc{}, fmt.Errorf("unknown context type for path %q", path)
	}
	return ctxResolvedDoc{Path: path, Type: t}, nil
}

// ctxFirstSegment returns the directory name right under context/ for a
// context-relative path ("context/plan/foo.md" -> "plan").
func ctxFirstSegment(path string) string {
	rel := strings.TrimPrefix(path, sdtWorkDir+string(filepath.Separator))
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	return parts[0]
}

// ctxFirstSegmentDir returns the full dir ("context/plan") for the first
// segment under context/.
func ctxFirstSegmentDir(path string) string {
	return filepath.Join(sdtWorkDir, ctxFirstSegment(path))
}

func resolveContextDocIdentity(cmd *cobra.Command) (ctxResolvedDoc, error) {
	typ := getStringFlag(cmd, "type", false)
	if typ == ctxDocAliasDecision {
		typ = ctxTypeDecision
	}
	if typ == "" {
		return ctxResolvedDoc{}, errors.New("document reference is required (a path or --type/--slug identity)")
	}
	t, ok := ctxTypeLookup(typ)
	if !ok {
		return ctxResolvedDoc{}, fmt.Errorf("unknown type %q (use %s)", typ, ctxTypeHelpText(ctxPathTypes()))
	}
	slug := sanitizeSlug(getStringFlag(cmd, "slug", false))

	var path string
	switch t.scheme {
	case ctxSchemeDecision:
		number := getStringFlag(cmd, "number", false)
		if number == "" {
			return ctxResolvedDoc{}, errors.New("--number is required for type decision")
		}
		if err := validateDecisionNumber(number); err != nil {
			return ctxResolvedDoc{}, err
		}
		if slug == "" {
			return ctxResolvedDoc{}, errors.New("--slug is required for type decision")
		}
		path = filepath.Join(t.dir, number+"-"+slug+sdtMarkdownExt)
	case ctxSchemePhase:
		if !t.pathSupported {
			return ctxResolvedDoc{}, fmt.Errorf("type %q cannot be addressed by identity", typ)
		}
		phase, plan, err := taskTarget(cmd)
		if err != nil {
			return ctxResolvedDoc{}, err
		}
		path = taskFileForRef(phase, taskStreamFlag(cmd), plan)
	case ctxSchemeTmpBySlug:
		if slug == "" {
			return ctxResolvedDoc{}, errors.New("--slug is required for type tmp")
		}
		path = filepath.Join(t.dir, slug)
	case ctxSchemeDated:
		if slug == "" {
			return ctxResolvedDoc{}, errors.New("--slug is required for type " + typ)
		}
		matches, err := filepath.Glob(filepath.Join(t.dir, "*-"+slug+sdtMarkdownExt))
		if err != nil {
			return ctxResolvedDoc{}, err
		}
		switch len(matches) {
		case 0:
			return ctxResolvedDoc{}, fmt.Errorf("no %s document with slug %q under %s/", typ, slug, t.dir)
		case 1:
			path = matches[0]
		default:
			return ctxResolvedDoc{}, fmt.Errorf("multiple %s documents match slug %q (%v); pass a full path instead", typ, slug, matches)
		}
	case ctxSchemeBare, ctxSchemeSubpath:
		if slug == "" {
			return ctxResolvedDoc{}, errors.New("--slug is required for type " + typ)
		}
		path = filepath.Join(t.dir, slug+sdtMarkdownExt)
	default:
		return ctxResolvedDoc{}, fmt.Errorf("type %q cannot be addressed by identity", typ)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ctxResolvedDoc{}, fmt.Errorf("no such document: %s", path)
		}
		return ctxResolvedDoc{}, err
	}
	if info.IsDir() {
		return ctxResolvedDoc{}, fmt.Errorf("%s is a directory", path)
	}
	return ctxResolvedDoc{Path: path, Type: t}, nil
}

// ctxStatusBearing reports whether a type carries a status field at all
// (worklog/notes/tmp do not).

func ctxStatusBearing(t ctxDocType) bool {
	return len(t.statuses) > 0
}

// ── context status get ─────────────────────────────────────────────────────────

type ctxStatusResult struct {
	Path    string `json:"path" yaml:"path"`
	Type    string `json:"type" yaml:"type"`
	Status  string `json:"status" yaml:"status"`
	Updated string `json:"updated,omitempty" yaml:"updated,omitempty"`
}

func outputContextStatus(cmd *cobra.Command, res ctxStatusResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(res, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		if res.Status == "" {
			res.Status = "(none)"
		}
		outputString(cmd, res.Status+"\n")
	}
}

var contextStatusGetCmd = &cobra.Command{
	Use:   "get <ref>",
	Short: "Print the frontmatter status of a context/ document",
	Long: `Print the frontmatter status field of an existing context/ document,
resolved by path (absolute or relative, with or without the .md suffix) or by
identity (--type/--slug, plus --number for decision or --phase/--plan for task
files).

Types without a status field (worklog, notes, tmp) are rejected.

Examples:
  sdt context status get context/plan/20260920-131900-plan-x.md
  sdt context status get plan/20260920-131900-plan-x
  sdt context status get --type decision --number 0001 --slug auth-choice
  sdt context status get --type analysis --slug backend --format json`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)
		if !ctxStatusBearing(doc.Type) {
			exitWithError(cmd, fmt.Errorf("type %s does not carry a status field (status get unsupported)", ctxKindLabel(doc.Type)))
		}
		data, err := os.ReadFile(doc.Path) //#nosec G304 -- user work file
		exitWithError(cmd, err)
		outputContextStatus(cmd, ctxStatusResult{
			Path:    doc.Path,
			Type:    doc.Type.kind,
			Status:  frontmatterField(string(data), ctxMapStatus),
			Updated: frontmatterField(string(data), "updated"),
		})
	},
}

// ── context status set ─────────────────────────────────────────────────────────

// frontmatterPatch describes a frontmatter key substitution. Patches applied
// in order; absent keys are appended before the closing `---`.

type frontmatterPatch struct {
	key   string
	value string
}

// setFrontmatterFields rewrites the frontmatter keys of content per patches,
// appending missing keys before the closing delimiter. It returns the new
// content and whether anything changed (contents without a frontmatter block
// are returned untouched).

func setFrontmatterFields(content string, patches []frontmatterPatch) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != ctxFrontmatterDelim {
		return content, false
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == ctxFrontmatterDelim {
			end = i
			break
		}
	}
	if end < 0 {
		return content, false
	}

	changed := false
	present := map[string]bool{}
	for i := 1; i < end; i++ {
		key := strings.SplitN(lines[i], ":", 2)[0]
		if key == "" {
			continue
		}
		present[key] = true
		for _, p := range patches {
			if p.key == key {
				if strings.TrimSpace(strings.TrimPrefix(lines[i], key+":")) != p.value {
					lines[i] = key + ": " + p.value
					changed = true
				}
			}
		}
	}

	var missing []string
	for _, p := range patches {
		if !present[p.key] {
			missing = append(missing, p.key+": "+p.value)
		}
	}
	if len(missing) > 0 {
		var tail []string
		tail = append(tail, lines[:end]...)
		tail = append(tail, missing...)
		tail = append(tail, lines[end:]...)
		lines = tail
		changed = true
	}
	if !changed {
		return content, false
	}
	return strings.Join(lines, "\n"), true
}

// frontmatterIsWellFormed reports whether content carries a closed frontmatter
// block. It lets a caller tell an idempotent no-op (every patched value already
// equals the stored one, `changed == false`) from a document with no usable
// frontmatter, so a same-value `touch`/`status set` succeeds instead of raising
// a false "missing frontmatter" error (F3).

func frontmatterIsWellFormed(content string) bool {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != ctxFrontmatterDelim {
		return false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == ctxFrontmatterDelim {
			return true
		}
	}
	return false
}

// ctxApplyFrontmatter is the one write path for a frontmatter transition on a
// resolved document: it reads the file, applies the patches, stamps a missing uid
// and the provenance, writes, and returns the status it replaced so a caller can
// report the transition. `context status set` and `context deprecate` both end
// here, so the `updated` refresh, the uid stamp and the provenance stamp cannot
// differ between them.
func ctxApplyFrontmatter(cmd *cobra.Command, doc ctxResolvedDoc, patches []frontmatterPatch) (string, error) {
	data, err := os.ReadFile(doc.Path) //#nosec G304 -- user work file
	if err != nil {
		return "", err
	}
	previous := parseFrontmatterField(string(data), ctxMapStatus)
	content, changed := setFrontmatterFields(string(data), patches)
	// Lazy identifier stamping: any rewrite of an eligible document also
	// stamps a missing uid (analysis Q3).
	if ctxUIDEligibleKind(doc.Type.kind) {
		if stamped, ok := stampUIDMissing(content); ok {
			content, changed = stamped, true
		}
	}
	if stamped := stampProvenance(content, cmd); stamped != content {
		content, changed = stamped, true
	}
	if !changed {
		if !frontmatterIsWellFormed(string(data)) {
			return "", fmt.Errorf("no status field written to %s (missing frontmatter)", doc.Path)
		}
		return previous, nil
	}
	//#nosec G306,G703 -- a resolved context/ work file
	if err := os.WriteFile(doc.Path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return previous, nil
}

// ctxStatusSetPatches builds the frontmatter patches for a status transition:
// the new status always; the refreshed `updated` timestamp when the kind's
// contract requires it (decisions never get an updated refresh).

func ctxStatusSetPatches(t ctxDocType, status string) []frontmatterPatch {
	patches := []frontmatterPatch{{key: ctxMapStatus, value: status}}
	if t.hasUpdated {
		patches = append(patches, frontmatterPatch{key: "updated", value: contextNow().UTC().Format(time.RFC3339)})
	}
	return patches
}

var contextStatusSetCmd = &cobra.Command{
	Use:   "set <ref> --status <v>",
	Short: "Set the frontmatter status of a context/ document",
	Long: `Set the frontmatter status field of an existing context/ document,
resolved by path or identity like ` + "`status get`" + `. The value is validated
against the kind's status vocabulary (from the instruction contracts). When
the kind requires ` + "`updated`" + ` it is refreshed to now.

Types without a status field (worklog, notes, tmp) are rejected. Decisions are
append-only: only the status field changes, never the body or filename.

Examples:
  sdt context status set context/analysis/20260920-130000-x.md --status archived
  sdt context status set --type decision --number 0001 --slug auth-choice --status accepted
  sdt context status set --type proposal --slug provenance --status review`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		status := getStringFlag(cmd, "status", true)
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)
		if !ctxStatusBearing(doc.Type) {
			exitWithError(cmd, fmt.Errorf("type %s does not carry a status field (status set unsupported)", ctxKindLabel(doc.Type)))
		}
		if !ctxStatusInVocab(doc.Type, status) {
			exitWithError(cmd, fmt.Errorf("invalid status %q for type %s (use %s)", status, doc.Type.kind, ctxStatusVocab(doc.Type)))
		}
		// A terminal status with a `--reason` is the close path: the reason is
		// recorded in the body (the same implementation `context close` uses), so
		// an early close is never silent — one verb per job, no second command.
		if reason := strings.TrimSpace(getStringFlag(cmd, "reason", false)); reason != "" && status == closeTerminalStatus(doc.Type) {
			if _, err := closeDocument(doc, true, reason); err != nil {
				exitWithError(cmd, err)
			}
			writeStatusResult(cmd, doc, status)
			cascadeAfterWrite(cmd, doc.Path)
			return
		}
		// Derived-state guard (analysis Q4): refuse a hand-declared terminal
		// status that contradicts the children/checklist unless --force.
		if status == taskFileStatusCompleted && !getBoolFlag(cmd, "force", false) {
			if _, reason := derivedCompletionBlock(doc.Path); reason != "" {
				exitWithError(cmd, fmt.Errorf("cannot set %s completed: %s (finish the work, or pass --force)", ctxKindLabel(doc.Type), reason))
			}
		}
		if _, err := ctxApplyFrontmatter(cmd, doc, ctxStatusSetPatches(doc.Type, status)); err != nil {
			exitWithError(cmd, err)
		}

		writeStatusResult(cmd, doc, status)
		cascadeAfterWrite(cmd, doc.Path)
	},
}

// writeStatusResult prints a status write in the selected format.
func writeStatusResult(cmd *cobra.Command, doc ctxResolvedDoc, status string) {
	updated := ""
	if doc.Type.hasUpdated {
		updated = contextNow().UTC().Format(time.RFC3339)
	}
	switch getFormat(cmd) {
	case fmtJSON, fmtYAML:
		res := ctxStatusResult{Path: doc.Path, Type: doc.Type.kind, Status: status, Updated: updated}
		if getFormat(cmd) == fmtJSON {
			out, err := json.MarshalIndent(res, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
			return
		}
		out, err := yaml.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		outputString(cmd, "ok\n")
	}
}

// ctxStatusInVocab reports whether v belongs to the kind's status vocabulary.

func ctxStatusInVocab(t ctxDocType, v string) bool {
	for _, s := range t.statuses {
		if s == v {
			return true
		}
	}
	return false
}

// ctxStatusVocab renders the pipe-joined vocabulary for error messages.

func ctxStatusVocab(t ctxDocType) string {
	return strings.Join(t.statuses, "|")
}

func addContextStatusRefFlags(c *cobra.Command) {
	c.Flags().String("type", "", "Document type for identity resolution")
	c.Flags().String("slug", "", "Slug for identity resolution")
	c.Flags().String("number", "", "Decision number (type decision)")
	c.Flags().String("phase", "", "Phase for a task file (type tasks)")
	c.Flags().String("stream", "", "Split-file label for a task file (type tasks)")
	c.Flags().String("plan", "", "Plan reference for a task file (type tasks)")
}

func init() {
	addContextStatusRefFlags(contextStatusGetCmd)
	addContextStatusRefFlags(contextStatusSetCmd)
	contextStatusSetCmd.Flags().String("status", "", "New status value (validated against the kind vocabulary)")
	contextStatusSetCmd.Flags().Bool("force", false, "Set a terminal status even when derived state disagrees")
	contextStatusSetCmd.Flags().String("reason", "", "With a terminal status: record why the derived state disagrees (the close path)")
	addCascadeFlag(contextStatusSetCmd)
	contextStatusCmd.AddCommand(contextStatusGetCmd, contextStatusSetCmd)
}
