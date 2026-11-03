package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/frontmatter"
)

// ── batch page creation (sdt context new --batch) ─────────────────────────────
//
// A batch file is data, never instructions: a YAML list of page descriptors,
// each carrying the fields `context new` plus `context set` would apply one by
// one. Every entry is validated before the first write, so a malformed entry
// cannot leave half a batch behind.

// ctxFrontmatterTags is the page tag list, shared by the batch writer.
const ctxFrontmatterTags = "tags"

// ctxFrontmatterType is the wiki page type key, shared by the batch writer.
const ctxFrontmatterType = "type"

// contextNewBatchKeys is the closed set of keys a batch entry may carry. A key
// outside it is refused by name rather than silently dropped, because a typo in
// a batch file would otherwise create a page missing a field.
var contextNewBatchKeys = []string{
	ctxFrontmatterKind, ctxDocTitleKey, "slug", ctxMapNameSummary, ctxFrontmatterType, ctxFrontmatterTags, ctxFrontmatterRelations, "source",
	"maturity", "confidence", "body",
}

// contextNewBatchEntry is one decoded page descriptor. Kind is the document
// type (`wiki`, `lesson`, `analysis`…) and Type is the wiki page's own `type`
// (`concept`, `entity`…), so the entry mirrors the frontmatter it writes.
type contextNewBatchEntry struct {
	Kind       string
	Title      string
	Slug       string
	Summary    string
	Type       string
	Tags       []string
	Relations  map[string][]string
	Sources    []string
	Maturity   string
	Confidence string
	Body       string
}

// contextNewBatchPlan is one entry resolved to the bytes that will be written.
type contextNewBatchPlan struct {
	Index   int
	Path    string
	Type    string
	Content string
}

// contextNewBatchResult is the machine-readable outcome of a batch run.
type contextNewBatchResult struct {
	Paths  []string `json:"paths" yaml:"paths"`
	DryRun bool     `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
}

// runContextNewBatch is the `context new --batch <file>` entry point.
func runContextNewBatch(cmd *cobra.Command, batchFile string) {
	for _, name := range []string{ctxFrontmatterType, ctxDocTitleKey, "slug", "input", "file", "inb64", ctxFrontmatterObjective, ctxMapStatus, "source", "note-type", "topic", "entity", "category", "number", "force", "edit"} {
		if cmd.Flags().Changed(name) {
			exitWithError(cmd, fmt.Errorf("--batch carries these fields per entry; drop --%s", name))
			return
		}
	}
	entries, err := readContextNewBatch(batchFile)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	plans, err := buildContextNewBatchPlan(cmd, entries)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	dryRun := getBoolFlag(cmd, "dry-run", false)
	if !dryRun {
		for _, plan := range plans {
			if err := writeContextNewBatchPlan(plan); err != nil {
				exitWithError(cmd, err)
				return
			}
		}
	}
	result := contextNewBatchResult{DryRun: dryRun}
	for _, plan := range plans {
		result.Paths = append(result.Paths, plan.Path)
	}
	outputContextNewBatch(cmd, result)
}

// readContextNewBatch decodes the YAML list and refuses any key outside the
// closed set, naming the entry index and the key.
func readContextNewBatch(path string) ([]contextNewBatchEntry, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- batch path named by the caller
	if err != nil {
		return nil, fmt.Errorf("read batch %s: %w", path, err)
	}
	var raw []map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse batch %s: %w", path, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("batch %s carries no entries", path)
	}
	entries := make([]contextNewBatchEntry, 0, len(raw))
	for i, m := range raw {
		entry, err := decodeContextNewBatchEntry(i, m)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// decodeContextNewBatchEntry validates the key set and the value shapes of one
// entry.
func decodeContextNewBatchEntry(idx int, m map[string]any) (contextNewBatchEntry, error) {
	var entry contextNewBatchEntry
	for key := range m {
		if !slices.Contains(contextNewBatchKeys, key) {
			return entry, fmt.Errorf("entry %d: unknown key %q (allowed: %s)", idx, key, strings.Join(contextNewBatchKeys, ", "))
		}
	}
	var err error
	if entry.Kind, err = batchStringField(idx, m, "kind"); err != nil {
		return entry, err
	}
	if entry.Title, err = batchStringField(idx, m, ctxDocTitleKey); err != nil {
		return entry, err
	}
	if entry.Slug, err = batchStringField(idx, m, "slug"); err != nil {
		return entry, err
	}
	if entry.Summary, err = batchStringField(idx, m, "summary"); err != nil {
		return entry, err
	}
	if entry.Type, err = batchStringField(idx, m, ctxFrontmatterType); err != nil {
		return entry, err
	}
	if entry.Maturity, err = batchStringField(idx, m, "maturity"); err != nil {
		return entry, err
	}
	if entry.Confidence, err = batchStringField(idx, m, "confidence"); err != nil {
		return entry, err
	}
	if entry.Body, err = batchStringField(idx, m, "body"); err != nil {
		return entry, err
	}
	if entry.Tags, err = batchStringListField(idx, m, ctxFrontmatterTags); err != nil {
		return entry, err
	}
	if entry.Sources, err = batchStringListField(idx, m, "source"); err != nil {
		return entry, err
	}
	if entry.Relations, err = batchRelationsField(idx, m, "relations"); err != nil {
		return entry, err
	}
	return entry, nil
}

// batchStringField reads a scalar field as a string, accepting the scalar
// spellings YAML produces for an unquoted value.
func batchStringField(idx int, m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", nil
	}
	switch t := v.(type) {
	case string:
		return t, nil
	case int, int64, float64, bool:
		return fmt.Sprint(t), nil
	default:
		return "", fmt.Errorf("entry %d: %q must be a scalar, got %T", idx, key, v)
	}
}

// batchStringListField reads a list field, accepting one scalar as a one-item
// list.
func batchStringListField(idx int, m map[string]any, key string) ([]string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	switch t := v.(type) {
	case string:
		return []string{t}, nil
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			switch s := item.(type) {
			case string:
				out = append(out, s)
			case int, int64, float64, bool:
				out = append(out, fmt.Sprint(s))
			default:
				return nil, fmt.Errorf("entry %d: %q must hold scalars, got %T", idx, key, item)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("entry %d: %q must be a list, got %T", idx, key, v)
	}
}

// batchRelationsField reads a verb → [[id|label]] map.
func batchRelationsField(idx int, m map[string]any, key string) (map[string][]string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	raw, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("entry %d: %q must be a map of verb to wikilinks, got %T", idx, key, v)
	}
	out := make(map[string][]string, len(raw))
	for verb, targets := range raw {
		items, err := batchStringListValue(idx, key, verb, targets)
		if err != nil {
			return nil, err
		}
		out[verb] = items
	}
	return out, nil
}

// batchStringListValue is the value half of batchRelationsField, split out so
// the map reader stays readable.
func batchStringListValue(idx int, key, verb string, v any) ([]string, error) {
	switch t := v.(type) {
	case string:
		return []string{t}, nil
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("entry %d: %s.%s must hold wikilink strings, got %T", idx, key, verb, item)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("entry %d: %s.%s must be a list, got %T", idx, key, verb, v)
	}
}

// buildContextNewBatchPlan resolves every entry to a path and its bytes before
// the first write. A failure returns no plan, so the batch is all-or-nothing.
func buildContextNewBatchPlan(cmd *cobra.Command, entries []contextNewBatchEntry) ([]contextNewBatchPlan, error) {
	agent := getStringFlag(cmd, "agent", false)
	role := getStringFlag(cmd, "role", false)
	created := contextNow().UTC().Format(time.RFC3339)
	project := ""
	if cfg, err := findProjectConfig(); err == nil && cfg != nil {
		project = cfg.Project
	}
	seen := map[string]int{}
	plans := make([]contextNewBatchPlan, 0, len(entries))
	for i, entry := range entries {
		plan, err := planContextNewBatchEntry(i, entry, agent, role, created, project)
		if err != nil {
			return nil, err
		}
		if prev, ok := seen[plan.Path]; ok {
			return nil, fmt.Errorf("entry %d: %s is also created by entry %d", i, plan.Path, prev)
		}
		seen[plan.Path] = i
		plans = append(plans, plan)
	}
	return plans, nil
}

// planContextNewBatchEntry applies the per-type rules `context new` enforces and
// renders the entry's bytes.
func planContextNewBatchEntry(idx int, e contextNewBatchEntry, agent, role, created, project string) (contextNewBatchPlan, error) {
	typ, title, slug, err := resolveBatchKindAndSlug(idx, e)
	if err != nil {
		return contextNewBatchPlan{}, err
	}
	sources, err := validateBatchEntryFields(idx, typ, slug, e)
	if err != nil {
		return contextNewBatchPlan{}, err
	}
	path, err := contextPath(typ, slug, "", "", "")
	if err != nil {
		return contextNewBatchPlan{}, fmt.Errorf("entry %d: %w", idx, err)
	}
	if _, err := os.Stat(path); err == nil {
		return contextNewBatchPlan{}, fmt.Errorf("entry %d: %s already exists (a batch never overwrites)", idx, path)
	} else if !os.IsNotExist(err) {
		return contextNewBatchPlan{}, fmt.Errorf("entry %d: %w", idx, err)
	}
	body, err := batchEntryBody(idx, typ, e)
	if err != nil {
		return contextNewBatchPlan{}, err
	}
	content, err := renderContextNewBatchEntry(typ, title, e, sources, slug, agent, role, project, created)
	if err != nil {
		return contextNewBatchPlan{}, fmt.Errorf("entry %d: %w", idx, err)
	}
	if body != "" {
		content += "\n" + strings.TrimRight(body, "\n") + "\n"
	}
	if typ == ctxTypePlan {
		content, _ = stampChecklistIDs(content)
	}
	return contextNewBatchPlan{Index: idx, Path: path, Type: typ, Content: content}, nil
}

// resolveBatchKindAndSlug applies the type and slug rules `context new` uses.
func resolveBatchKindAndSlug(idx int, e contextNewBatchEntry) (typ, title, slug string, err error) {
	typ = strings.TrimSpace(e.Kind)
	if typ == "" {
		return "", "", "", fmt.Errorf("entry %d: missing `kind`", idx)
	}
	t, ok := ctxTypeLookup(typ)
	if !ok || !t.newSupported {
		return "", "", "", fmt.Errorf("entry %d: new supports type %s, got %q", idx, ctxTypeHelpText(ctxNewTypes()), typ)
	}
	if typ == ctxTypeDecision {
		return "", "", "", fmt.Errorf("entry %d: a decision is numbered on write; create it with `context new --type decision` outside a batch", idx)
	}
	slug = sanitizeSlug(e.Slug)
	if ctxTypeTakesSubpathSlug(typ) && e.Slug != "" {
		cleaned, cerr := ctxCleanSlug(e.Slug, true)
		if cerr != nil {
			return "", "", "", fmt.Errorf("entry %d: %w", idx, cerr)
		}
		slug = cleaned
	}
	title = strings.TrimSpace(e.Title)
	if slug == "" && title != "" {
		slug = sanitizeSlug(title)
	}
	if slug == "" {
		return "", "", "", fmt.Errorf("entry %d: `title` or `slug` is required", idx)
	}
	return typ, title, slug, nil
}

// validateBatchEntryFields checks the fields that depend on the resolved kind
// and slug.
func validateBatchEntryFields(idx int, typ, slug string, e contextNewBatchEntry) ([]string, error) {
	sources := sanitizeReferenceList(e.Sources)
	if typ == ctxTypeLesson {
		pageRef := "wiki/" + slug + sdtMarkdownExt
		if len(sources) == 0 {
			sources = []string{pageRef}
		} else if !slices.Contains(sources, pageRef) {
			return nil, fmt.Errorf("entry %d: a lesson cites its concept page: `source` must include %q, got %s", idx, pageRef, strings.Join(sources, ", "))
		}
	}
	if e.Type != "" {
		if typ != ctxTypeWiki {
			return nil, fmt.Errorf("entry %d: `type` is the wiki page type; kind %s carries none", idx, typ)
		}
		if !contextwiki.Types[e.Type] {
			return nil, fmt.Errorf("entry %d: unknown wiki `type` %q (use concept|entity|decision|pattern|module)", idx, e.Type)
		}
	}
	if e.Maturity != "" && !ctxMaturityValid(e.Maturity) {
		return nil, fmt.Errorf("entry %d: `maturity` %q is outside the vocabulary (%s)", idx, e.Maturity, strings.Join(ctxMaturityValues, " | "))
	}
	if err := validateBatchRelations(idx, e.Relations); err != nil {
		return nil, err
	}
	return sources, nil
}

// batchEntryBody reads the entry's body path or falls back to the kind default.
func batchEntryBody(idx int, typ string, e contextNewBatchEntry) (string, error) {
	if e.Body == "" {
		return contextDefaultBody(typ), nil
	}
	data, err := readBatchBody(e.Body)
	if err != nil {
		return "", fmt.Errorf("entry %d: %w", idx, err)
	}
	return string(data), nil
}

// validateBatchRelations checks the shape of the relations map: a closed verb
// and a [[id|label]] target. Target resolution is the lint's job — a batch may
// legitimately relate pages it is creating in the same run.
func validateBatchRelations(idx int, relations map[string][]string) error {
	for verb, targets := range relations {
		if verb == "" || !contextwiki.Verbs[verb] {
			return fmt.Errorf("entry %d: relation verb %q is outside the closed vocabulary", idx, verb)
		}
		for _, target := range targets {
			if contextwiki.LinkRegexp.FindStringSubmatch(target) == nil {
				return fmt.Errorf("entry %d: relation %s target %q is not a [[id|label]] link", idx, verb, target)
			}
		}
	}
	return nil
}

// readBatchBody reads a body path inside context/ and refuses an absolute path
// or one that escapes the corpus.
func readBatchBody(ref string) ([]byte, error) {
	if filepath.IsAbs(ref) {
		return nil, fmt.Errorf("body %q must be a path inside context/", ref)
	}
	clean := filepath.Clean(ref)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("body %q escapes context/", ref)
	}
	root := filepath.Clean(sdtWorkDir)
	abs := filepath.Join(root, clean)
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return nil, fmt.Errorf("body %q escapes context/", ref)
	}
	data, err := os.ReadFile(abs) //#nosec G304 -- path checked against the context/ root
	if err != nil {
		return nil, fmt.Errorf("body %q: %w", ref, err)
	}
	return data, nil
}

// renderContextNewBatchEntry builds the frontmatter and applies the fields the
// single path applies through `context set`, using the same writer.
func renderContextNewBatchEntry(typ, title string, e contextNewBatchEntry, sources []string, slug, agent, role, project, created string) (string, error) {
	component := ""
	if typ == ctxTypeArchitecture || typ == ctxTypeBriefing {
		component = slug
	}
	content := contextFrontmatter(typ, title, e.Summary, "", project, component, created, "", slug, agent, role, "", "", nil, nil, nil)
	if len(sources) > 0 {
		content = injectReferenceList(content, ctxFrontmatterLinks, sources)
		content = injectReferenceList(content, ctxFrontmatterSources, sources)
	}
	var err error
	if e.Type != "" {
		if content, err = setGeneratedFrontmatter(content, ctxFrontmatterType, frontmatter.Scalar(e.Type)); err != nil {
			return "", err
		}
	}
	if len(e.Tags) > 0 {
		if content, err = setGeneratedFrontmatterList(content, ctxFrontmatterTags, e.Tags); err != nil {
			return "", err
		}
	}
	if len(e.Relations) > 0 {
		if content, err = setGeneratedFrontmatter(content, ctxFrontmatterRelations, renderBatchRelations(e.Relations)); err != nil {
			return "", err
		}
	}
	if e.Maturity != "" {
		if content, err = setGeneratedFrontmatter(content, ctxFrontmatterMaturity, frontmatter.Scalar(e.Maturity)); err != nil {
			return "", err
		}
	}
	if e.Confidence != "" {
		if content, err = setGeneratedFrontmatter(content, "confidence", frontmatter.Scalar(e.Confidence)); err != nil {
			return "", err
		}
	}
	return content, nil
}

// renderBatchRelations renders the nested map the `relations` block holds.
func renderBatchRelations(relations map[string][]string) string {
	verbs := make([]string, 0, len(relations))
	for verb := range relations {
		verbs = append(verbs, verb)
	}
	slices.Sort(verbs)
	var b strings.Builder
	for i, verb := range verbs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  " + verb + ":")
		for _, target := range relations[verb] {
			b.WriteString("\n    - " + frontmatter.Scalar(target))
		}
	}
	return b.String()
}

// setGeneratedFrontmatter writes one scalar key of a generated frontmatter
// block, preserving the block's other bytes.
func setGeneratedFrontmatter(content, key, value string) (string, error) {
	block, err := frontmatter.Parse(content)
	if err != nil {
		return "", err
	}
	newYAML, err := block.Set(key, value, ctxKeyOrder(""))
	if err != nil {
		return "", err
	}
	return block.ReplaceBody(content, newYAML), nil
}

// setGeneratedFrontmatterList writes one list key of a generated frontmatter
// block as a block sequence.
func setGeneratedFrontmatterList(content, key string, items []string) (string, error) {
	block, err := frontmatter.Parse(content)
	if err != nil {
		return "", err
	}
	quoted := make([]string, 0, len(items))
	for _, item := range items {
		quoted = append(quoted, frontmatter.Scalar(item))
	}
	newYAML, err := block.SetList(key, quoted, frontmatter.ShapeBlock, ctxKeyOrder(""))
	if err != nil {
		return "", err
	}
	return block.ReplaceBody(content, newYAML), nil
}

// writeContextNewBatchPlan creates the plan's parent directory and file.
func writeContextNewBatchPlan(plan contextNewBatchPlan) error {
	dir, _ := filepath.Split(plan.Path)
	if err := os.MkdirAll(dir, 0o750); err != nil { //#nosec G301 -- user work dir
		return err
	}
	if err := os.WriteFile(plan.Path, []byte(plan.Content), 0o644); err != nil { //#nosec G306 -- user work file
		return err
	}
	return nil
}

// outputContextNewBatch renders the batch outcome.
func outputContextNewBatch(cmd *cobra.Command, res contextNewBatchResult) {
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
		for _, path := range res.Paths {
			outputString(cmd, path+"\n")
		}
	}
}
