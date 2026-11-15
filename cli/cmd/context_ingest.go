package cmd

import (
	cryptosha "crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/distill"
	"github.com/sandrolain/sdt/internal/sourceclass"
)

// ── ingestion queue (sdt context ingest) ─────────────────────────────────────
//
// The binaries in context/ingestion/ cannot carry YAML frontmatter, so the
// pending/archived markers the doctrine asks for live in a derived manifest and
// an archive sidecar instead. The command never converts and never distils: it
// makes the queue and the archive visible and safe.

const (
	ingestManifestName    = "manifest.yaml"
	ingestSidecarSuffix   = ".source.yaml"
	ingestStatusPending   = "pending"
	ingestStatusConverted = "converted"
	ingestStatusArchived  = "archived"
)

type ingestEntry struct {
	Path      string `json:"path" yaml:"path"`
	Bytes     int64  `json:"bytes" yaml:"bytes"`
	SHA256    string `json:"sha256" yaml:"sha256"`
	Status    string `json:"status" yaml:"status"`
	Converted string `json:"converted,omitempty" yaml:"converted,omitempty"`
	Archived  string `json:"archived,omitempty" yaml:"archived,omitempty"`
}

type ingestManifest struct {
	Generated string        `json:"generated" yaml:"generated"`
	Sources   []ingestEntry `json:"sources" yaml:"sources"`
}

type ingestArchiveRecord struct {
	Source string `yaml:"source"`
	// OriginalName is set only when the archived file carried a name an agent
	// runtime resolves as instructions: the stored file then ends in
	// `.source`, and this field keeps the name it arrived with.
	OriginalName string `yaml:"original_name,omitempty"`
	Bytes        int64  `yaml:"bytes"`
	SHA256       string `yaml:"sha256"`
	Converted    string `yaml:"converted,omitempty"`
	// Classification and License are what the source declares about its own use.
	// A page inherits them (strictest wins), and the release respects them.
	Classification string `yaml:"classification,omitempty"`
	License        string `yaml:"license,omitempty"`
	ArchivedAt     string `yaml:"archived_at"`
}

var contextIngestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Track sources in context/ingestion/ (list, status, archive, plan)",
	Long: `Track the raw sources that wait in context/ingestion/.

A binary source cannot carry frontmatter, so the pending marker is the derived
manifest (context/ingestion/manifest.yaml), not a YAML block inside the EPUB.
The commands never convert or distil:

  list    scan context/ingestion/ and derive pending/converted/archived
  status  print one source's derived entry
  archive move a source to context/refs/ after a hash check
  plan    print the source, route, counts, scope and merge|reset mode`,
}

var contextIngestListCmd = &cobra.Command{
	Use:   "list",
	Short: "List context/ingestion/ sources with a derived status",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		entries, err := ingestEntries()
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		if getBoolFlag(cmd, "write", false) {
			if err := writeIngestManifest(entries); err != nil {
				exitWithError(cmd, err)
				return
			}
		}
		outputIngestEntries(cmd, entries)
	},
}

var contextIngestStatusCmd = &cobra.Command{
	Use:   "status <source>",
	Short: "Print one ingestion source's derived status",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path, err := resolveIngestSource(args[0], true)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		entry, err := ingestEntryFor(path)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		outputIngestEntry(cmd, entry)
	},
}

var contextIngestArchiveCmd = &cobra.Command{
	Use:   "archive <source>",
	Short: "Move an ingestion source to context/refs/ after a hash check",
	Long: `Move one source from context/ingestion/ to context/refs/<basename>.

The destination must not exist. The source's sha256 is checked before the move;
an unconverted source is refused unless --force is set, because archiving is the
step after distillation. A sidecar <basename>.source.yaml records the hash and
the converted artifact, so the archive is auditable without frontmatter on the
binary. A source whose name an agent runtime resolves as instructions (AGENTS.md,
CLAUDE.md, GEMINI.md, SKILL.md and the other names in the closed list) is stored
as <basename>.source and the sidecar records original_name: a third party's
instructions must not arrive loadable. --dry-run prints the plan and writes
nothing.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path, err := resolveIngestSource(args[0], false)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		if !pathUnder(path, sdtIngestionDir) {
			exitWithError(cmd, fmt.Errorf("%s is not a source under %s/", path, sdtIngestionDir))
			return
		}
		sha, bytes, err := fileSHA256(path)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		converted, err := findConvertedSource(path)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		// A markdown source is its own artifact: there is nothing to convert, and
		// demanding one would only teach the caller to pass --force.
		markdownSource := strings.HasSuffix(strings.ToLower(path), sdtMarkdownExt)
		if converted == "" && !markdownSource && !getBoolFlag(cmd, "force", false) {
			exitWithError(cmd, fmt.Errorf("%s has no matching converted artifact; archive it with --force only after the run is dispositioned", corpusRelative(path)))
			return
		}
		// Validate before touching the filesystem: a refused flag must leave the
		// source where it is.
		classification := getStringFlag(cmd, "classification", false)
		if classification != "" && !sourceclass.Known(classification) {
			exitWithError(cmd, fmt.Errorf("--classification %q is not one of %s", classification, strings.Join(sourceclass.Tiers(), ", ")))
			return
		}
		dest := filepath.Join(sdtRefsDir, filepath.Base(path))
		originalName := ""
		if isSourceInstructionName(filepath.Base(dest)) {
			originalName = filepath.Base(dest)
			dest += sourceInstructionArchiveSuffix
		}
		if _, statErr := os.Stat(dest); statErr == nil {
			exitWithError(cmd, fmt.Errorf("archive destination %s already exists", corpusRelative(dest)))
			return
		}
		if getBoolFlag(cmd, "dry-run", false) {
			outputString(cmd, fmt.Sprintf("would archive: %s -> %s (sha256 %s)\n", corpusRelative(path), corpusRelative(dest), sha[:12]))
			if slug := sanitizeSlug(strings.TrimSuffix(filepath.Base(dest), filepath.Ext(dest))); slug != "" {
				outputString(cmd, fmt.Sprintf("would write sheet: %s\n", corpusRelative(filepath.Join(sdtSourcesDir, slug+sdtMarkdownExt))))
			}
			return
		}
		// A project may archive before it ever had a refs/ directory.
		if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			exitWithError(cmd, fmt.Errorf("archive %s: %w", corpusRelative(dest), err))
			return
		}
		if err := os.Rename(path, dest); err != nil {
			exitWithError(cmd, fmt.Errorf("archive %s: %w", corpusRelative(path), err))
			return
		}
		record := ingestArchiveRecord{
			Source:         corpusRelative(path),
			OriginalName:   originalName,
			Classification: classification,
			License:        getStringFlag(cmd, "license", false),
			Bytes:          bytes,
			SHA256:         sha,
			Converted:      converted,
			ArchivedAt:     time.Now().UTC().Format(time.RFC3339),
		}
		out, merr := yaml.Marshal(record)
		if merr != nil {
			exitWithError(cmd, merr)
			return
		}
		sidecarBase := filepath.Base(dest)
		if originalName != "" {
			sidecarBase = originalName
		}
		sidecar := filepath.Join(filepath.Dir(dest), sidecarBase+ingestSidecarSuffix)
		if werr := os.WriteFile(sidecar, out, 0o600); werr != nil { //#nosec G306 -- provenance, not a secret but 0600 is the project convention
			exitWithError(cmd, werr)
			return
		}
		if _, statErr := os.Stat(filepath.Join(sdtIngestionDir, ingestManifestName)); statErr == nil {
			if entries, lerr := ingestEntries(); lerr == nil {
				if werr := writeIngestManifest(entries); werr != nil {
					fmt.Fprintf(os.Stderr, "warning: refresh ingestion manifest: %v\n", werr)
				}
			}
		}
		// The custody join (decision 0030): the kept artifact's provenance
		// follows the move and the sheet is created or linked in the same step,
		// so an archive never produces a provenance or a citation surface
		// without a custody record again.
		if converted != "" {
			rewrote, rerr := rewriteConvertedSource(converted, sha, corpusRelative(dest))
			if rerr != nil {
				fmt.Fprintf(os.Stderr, "warning: rewrite provenance of %s: %v\n", converted, rerr)
			} else if rewrote {
				outputString(cmd, fmt.Sprintf("provenance: %s source -> %s\n", converted, corpusRelative(dest)))
			}
		}
		sheetPath, sheetAction, serr := archiveSourceSheet(corpusRelative(dest), record, converted)
		switch {
		case serr != nil:
			fmt.Fprintf(os.Stderr, "warning: source sheet: %v\n", serr)
		case sheetAction == "":
			fmt.Fprintf(os.Stderr, "warning: source sheet not written — %s records other bytes and its digest-suffixed name is taken\n", sheetPath)
		default:
			outputString(cmd, fmt.Sprintf("sheet: %s %s\n", sheetAction, sheetPath))
		}
		outputString(cmd, fmt.Sprintf("archived: %s -> %s\n", corpusRelative(path), corpusRelative(dest)))
	},
}

type ingestRunPlan struct {
	Source        string `json:"source" yaml:"source"`
	Status        string `json:"status" yaml:"status"`
	Artifact      string `json:"artifact" yaml:"artifact"`
	Route         string `json:"route" yaml:"route"`
	RouteHeadings int    `json:"route_headings" yaml:"route_headings"`
	Lines         int    `json:"lines" yaml:"lines"`
	CoverageUnits int    `json:"coverage_units" yaml:"coverage_units"`
	Coverage      string `json:"coverage" yaml:"coverage"`
	Inventory     string `json:"inventory" yaml:"inventory"`
	Scope         string `json:"scope" yaml:"scope"`
	Mode          string `json:"mode" yaml:"mode"`
	Archive       string `json:"archive" yaml:"archive"`
	Created       string `json:"created" yaml:"created"`
}

var contextIngestPlanCmd = &cobra.Command{
	Use:   "plan <source>",
	Short: "Print the source, route, counts, scope and merge|reset mode",
	Long: `Print the run plan for one ingestion source.

The command is read-only unless --write is set. It derives the source status;
with --artifact it reads the converted artifact and prints the routing counts.
Coverage counters need the pages' citations and stay "not audited" here: run
sdt context wiki coverage (or sdt context wiki audit) for the countable block.
--scope records the bounded units the run will read; --mode records whether the
run merges into the corpus or resets the source's pages first. --write persists
the same plan under context/ingestion/runs/<UTC timestamp>.yaml and never edits
the source or a page.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path, err := resolveIngestSource(args[0], true)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		entry, err := ingestEntryFor(path)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		mode := getStringFlag(cmd, "mode", false)
		if mode != "merge" && mode != "reset" {
			exitWithError(cmd, fmt.Errorf("--mode must be merge or reset, got %q", mode))
			return
		}
		scope := getStringFlag(cmd, "scope", false)
		if scope == "" {
			scope = "all"
		}
		artifactRef := getStringFlag(cmd, "artifact", false)
		route, headings, lines, units := learnProvenanceUnknown, 0, 0, 0
		if artifactRef != "" {
			artifact, aerr := resolveCorpusFile(artifactRef)
			if aerr != nil {
				exitWithError(cmd, aerr)
				return
			}
			data, rerr := os.ReadFile(artifact) //#nosec G304 -- corpus path resolved under context/
			if rerr != nil {
				exitWithError(cmd, rerr)
				return
			}
			parsed, stats := distill.Parse(string(data), distill.Options{})
			route = routeFor(stats)
			headings = stats.Headings23
			lines = stats.Lines
			for _, u := range parsed {
				if u.Kind != distill.UnitContainer {
					units++
				}
			}
			artifactRef = corpusRelative(artifact)
		}
		now := time.Now().UTC()
		plan := ingestRunPlan{
			Source:        entry.Path,
			Status:        entry.Status,
			Artifact:      artifactRef,
			Route:         route,
			RouteHeadings: headings,
			Lines:         lines,
			CoverageUnits: units,
			Coverage:      "not audited",
			Inventory:     getStringFlag(cmd, "inventory", false),
			Scope:         scope,
			Mode:          mode,
			Archive:       entry.Archived,
			Created:       now.Format(time.RFC3339),
		}
		var b strings.Builder
		fmt.Fprintf(&b, "source: %s\n", plan.Source)
		fmt.Fprintf(&b, "status: %s\n", plan.Status)
		fmt.Fprintf(&b, "artifact: %s\n", emptyDash(plan.Artifact))
		fmt.Fprintf(&b, "route: %s\n", plan.Route)
		fmt.Fprintf(&b, "route_headings: %d\n", plan.RouteHeadings)
		fmt.Fprintf(&b, "lines: %d\n", plan.Lines)
		fmt.Fprintf(&b, "coverage_units: %d\n", plan.CoverageUnits)
		fmt.Fprintf(&b, "coverage: %s\n", plan.Coverage)
		fmt.Fprintf(&b, "inventory: %s\n", emptyDash(plan.Inventory))
		fmt.Fprintf(&b, "scope: %s\n", plan.Scope)
		fmt.Fprintf(&b, "mode: %s\n", plan.Mode)
		fmt.Fprintf(&b, "archive: %s\n", emptyDash(plan.Archive))
		fmt.Fprintf(&b, "created: %s\n", plan.Created)
		if getBoolFlag(cmd, "write", false) {
			manifestPath, werr := writeIngestRunPlan(now, plan)
			if werr != nil {
				exitWithError(cmd, werr)
				return
			}
			fmt.Fprintf(&b, "run manifest: %s\n", corpusRelative(manifestPath))
		}
		outputString(cmd, b.String())
	},
}

func writeIngestRunPlan(now time.Time, plan ingestRunPlan) (string, error) {
	dir := filepath.Join(sdtIngestionDir, "runs")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	out, err := yaml.Marshal(plan)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, now.Format("20060102T150405.000000000Z")+".yaml")
	if err := os.WriteFile(path, out, 0o644); err != nil { //#nosec G306 -- derived run plan
		return "", err
	}
	return path, nil
}

func ingestEntries() ([]ingestEntry, error) {
	var out []ingestEntry
	err := filepath.WalkDir(sdtIngestionDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "runs" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") || name == ingestManifestName || strings.HasSuffix(name, ingestSidecarSuffix) {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil || !info.Mode().IsRegular() {
			return nil
		}
		e, eerr := ingestEntryFor(path)
		if eerr != nil {
			return eerr
		}
		out = append(out, e)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return []ingestEntry{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", sdtIngestionDir, err)
	}
	return out, nil
}

func ingestEntryFor(path string) (ingestEntry, error) {
	sha, bytes, err := fileSHA256(path)
	if err != nil {
		return ingestEntry{}, err
	}
	entry := ingestEntry{Path: corpusRelative(path), Bytes: bytes, SHA256: sha, Status: ingestStatusPending}
	if strings.HasPrefix(filepath.Clean(path), filepath.Clean(sdtRefsDir)+string(filepath.Separator)) {
		entry.Status = ingestStatusArchived
		entry.Archived = corpusRelative(path)
		return entry, nil
	}
	archived := filepath.Join(sdtRefsDir, filepath.Base(path))
	if _, err := os.Stat(archived); err == nil {
		entry.Status = ingestStatusArchived
		entry.Archived = corpusRelative(archived)
		return entry, nil
	}
	converted, err := findConvertedSource(path)
	if err != nil {
		return ingestEntry{}, err
	}
	if converted != "" {
		entry.Status = ingestStatusConverted
		entry.Converted = converted
	}
	return entry, nil
}

func writeIngestManifest(entries []ingestEntry) error {
	if err := os.MkdirAll(sdtIngestionDir, 0o750); err != nil {
		return err
	}
	manifest := ingestManifest{
		Generated: time.Now().UTC().Format(time.RFC3339),
		Sources:   entries,
	}
	out, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sdtIngestionDir, ingestManifestName), out, 0o644) //#nosec G306 -- derived queue manifest
}

func outputIngestEntries(cmd *cobra.Command, entries []ingestEntry) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(entries, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(entries)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		var b strings.Builder
		b.WriteString("source\tbytes\tsha256\tstatus\tconverted\n")
		for _, e := range entries {
			fmt.Fprintf(&b, "%s\t%d\t%s\t%s\t%s\n", e.Path, e.Bytes, shortSHA(e.SHA256), e.Status, emptyDash(e.Converted))
		}
		outputString(cmd, b.String())
	}
}

func outputIngestEntry(cmd *cobra.Command, entry ingestEntry) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(entry, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(entry)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "source: %s\n", entry.Path)
		fmt.Fprintf(&b, "bytes: %d\n", entry.Bytes)
		fmt.Fprintf(&b, "sha256: %s\n", entry.SHA256)
		fmt.Fprintf(&b, "status: %s\n", entry.Status)
		fmt.Fprintf(&b, "converted: %s\n", emptyDash(entry.Converted))
		fmt.Fprintf(&b, "archived: %s\n", emptyDash(entry.Archived))
		outputString(cmd, b.String())
	}
}

// resolveIngestSource resolves a source under context/ingestion/ (or, for
// status, under context/refs/). It refuses the manifest and sidecars.
func resolveIngestSource(ref string, allowRefs bool) (string, error) {
	candidates := []string{ref, filepath.Join(sdtWorkDir, ref), filepath.Join(sdtIngestionDir, ref)}
	if allowRefs {
		candidates = append(candidates, filepath.Join(sdtRefsDir, ref))
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if !pathUnder(candidate, sdtIngestionDir) && (!allowRefs || !pathUnder(candidate, sdtRefsDir)) {
			return "", fmt.Errorf("%q is outside %s/ and %s/", ref, sdtIngestionDir, sdtRefsDir)
		}
		base := filepath.Base(candidate)
		if base == ingestManifestName || strings.HasSuffix(base, ingestSidecarSuffix) {
			return "", fmt.Errorf("%q is a manifest or sidecar, not a source", ref)
		}
		return candidate, nil
	}
	if found, err := findIngestSourceByName(ref, allowRefs); err == nil {
		return found, nil
	} else if !strings.Contains(err.Error(), "not found") {
		return "", err
	}
	return "", fmt.Errorf("source %q not found under %s/", ref, sdtIngestionDir)
}

func findIngestSourceByName(ref string, allowRefs bool) (string, error) {
	roots := []string{sdtIngestionDir}
	if allowRefs {
		roots = append(roots, sdtRefsDir)
	}
	var matches []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == ingestManifestName || strings.HasSuffix(name, ingestSidecarSuffix) {
				return nil
			}
			if name == ref || corpusRelative(path) == ref {
				matches = append(matches, path)
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("source %q not found", ref)
	default:
		return "", fmt.Errorf("source %q is ambiguous: %s", ref, strings.Join(matches, ", "))
	}
}

func pathUnder(path, root string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rabs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	return abs != rabs && strings.HasPrefix(abs, rabs+string(filepath.Separator))
}

func findConvertedSource(source string) (string, error) {
	sha, _, err := fileSHA256(source)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(sdtConvertedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(sdtConvertedDir, entry.Name())
		data, rerr := os.ReadFile(path) //#nosec G304 -- converted corpus path
		if rerr != nil {
			continue
		}
		front := string(data)
		src := contextwiki.FrontmatterField(front, "source")
		if src != "" && sameSourcePath(src, source) {
			return corpusRelative(path), nil
		}
		// doc2md's provenance carries the source binary's sha256 even when the
		// recorded source path is an intermediate text file.
		if got := contextwiki.FrontmatterField(front, "sha256"); got != "" && got == sha {
			return corpusRelative(path), nil
		}
	}
	return "", nil
}

func sameSourcePath(a, b string) bool {
	ca, cb := filepath.Clean(a), filepath.Clean(b)
	if ca == cb {
		return true
	}
	if filepath.Base(ca) == filepath.Base(cb) {
		return true
	}
	return corpusRelative(ca) == corpusRelative(cb)
}

func fileSHA256(path string) (string, int64, error) {
	f, err := os.Open(path) //#nosec G304 -- corpus path resolved by the caller
	if err != nil {
		return "", 0, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "warning: close %s: %v\n", path, cerr)
		}
	}()
	h := cryptosha.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), n, nil
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// ── the custody join (decision 0030) ─────────────────────────────────────────
//
// The archive move is where a source stops living where it was converted from.
// Two records must follow it in the same step or the corpus re-learns the same
// break later: the kept artifact's `source:` header (which would otherwise name
// a path the move erases) and the source sheet (the custody record with the
// pin, the extract and the restore recipe). Both are written by the verb, never
// hand-shaped afterwards.

// rewriteConvertedSource points a kept artifact's `source:` header at the
// archived copy. Only that one line moves: the body bytes, and the ~700
// citations pinned to them, stay exactly where they are. The rewrite fires when
// the artifact's provenance digest names this source — either the header still
// names the pre-archive path or it names an intermediate file matched only by
// digest (the `context/tmp/` provenance the two converted books carried).
func rewriteConvertedSource(converted, sha256, archivedCorpus string) (rewritten bool, retErr error) {
	rel := filepath.FromSlash(converted)
	if !filepath.IsLocal(rel) {
		return false, fmt.Errorf("converted artifact path %q is not corpus-relative", converted)
	}
	root, err := os.OpenRoot(sdtWorkDir)
	if err != nil {
		return false, err
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			retErr = errors.Join(retErr, closeErr)
		}
	}()
	data, err := root.ReadFile(rel)
	if err != nil {
		return false, err
	}
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return false, nil
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return false, nil
	}
	fm, rest := content[:end+4+1], content[end+4+1:]
	if contextwiki.FrontmatterField(fm, "sha256") != sha256 {
		return false, nil
	}
	if contextwiki.FrontmatterField(fm, "source") == archivedCorpus {
		return false, nil
	}
	updated, found := replaceFrontmatterKeyLine(fm, "source", archivedCorpus)
	if !found {
		return false, nil
	}
	if err := root.WriteFile(rel, []byte(updated+rest), 0o644); err != nil {
		return false, err
	}
	oldSum := cryptosha.Sum256(data)
	newSum := cryptosha.Sum256([]byte(updated + rest))
	if _, err := rewriteCitationDigest(converted, hex.EncodeToString(oldSum[:]), hex.EncodeToString(newSum[:])); err != nil {
		return true, fmt.Errorf("provenance updated but citation re-pin failed: %w", err)
	}
	return true, nil
}

// rewriteCitationDigest updates only citations whose path and digest prefix
// identify the converted artifact's previous bytes. Rewriting its provenance
// changes the file hash even though its cited body and line offsets stay fixed.
func rewriteCitationDigest(ref, oldSHA, newSHA string) (int, error) {
	changed := 0
	citationRef := strings.TrimPrefix(filepath.ToSlash(ref), "context/")
	dirs := append(append([]string(nil), ctxIndexDirs...), sdtWikiDir)
	for _, dir := range dirs {
		files, err := dirFilesFor(dir)
		if err != nil {
			continue
		}
		root, err := os.OpenRoot(dir)
		if err != nil {
			continue
		}
		for _, path := range files {
			rel, relErr := filepath.Rel(dir, path)
			if relErr != nil {
				continue
			}
			data, err := root.ReadFile(rel)
			if err != nil {
				continue
			}
			body := string(data)
			updated := ctxCitationRegexp.ReplaceAllStringFunc(body, func(match string) string {
				m := ctxCitationRegexp.FindStringSubmatch(match)
				if len(m) != 4 || m[1] != citationRef || !strings.HasPrefix(oldSHA, m[3]) {
					return match
				}
				changed++
				return strings.Replace(match, m[3], newSHA[:len(m[3])], 1)
			})
			if updated != body {
				if err := root.WriteFile(rel, []byte(updated), 0o644); err != nil {
					return changed, err
				} //#nosec G306 -- corpus documents use the repository's standard mode
			}
		}
		if err := root.Close(); err != nil {
			return changed, err
		}
	}
	return changed, nil
}

// replaceFrontmatterKeyLine rewrites the first top-level `key: …` line of a
// frontmatter block and reports whether the key was there; every other byte,
// including comments and quoting on other keys, is preserved.
func replaceFrontmatterKeyLine(fm, key, value string) (string, bool) {
	lines := strings.Split(fm, "\n")
	prefix := key + ":"
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = prefix + " " + value
			return strings.Join(lines, "\n"), true
		}
	}
	return fm, false
}

// archiveSourceSheet creates or links the custody sheet (`context/sources/`)
// for an archived source: `capture_path`, `extract`, `sha256`, `checked_at`,
// with the restore recipe in the custody notes (the frontmatter `restore` key
// belongs to the `gone` transition). A sheet already pinning the same digest or
// capture path is linked — only keys it lacks are filled; a sheet recording
// other bytes is never clobbered, the new record then lands on a
// digest-suffixed slug. The returned action is `created`, `linked`, or empty
// when nothing was written because both slugs record other bytes.
func archiveSourceSheet(archivedCorpus string, record ingestArchiveRecord, converted string) (string, string, error) {
	base := strings.TrimSuffix(filepath.Base(archivedCorpus), filepath.Ext(archivedCorpus))
	slug := sanitizeSlug(base)
	if slug == "" {
		return "", "", nil
	}
	if err := os.MkdirAll(sdtSourcesDir, 0o750); err != nil {
		return "", "", err
	}
	keys := ingestSheetKeys(archivedCorpus, record, converted)
	plain := filepath.Join(sdtSourcesDir, slug+sdtMarkdownExt)
	content, exists, err := readSourceSheet(plain)
	if err != nil {
		return "", "", err
	}
	action := ""
	if exists && !sheetPinsSource(content, archivedCorpus, record.SHA256) {
		// The plain slug records other bytes: the digest-suffixed name is the
		// last candidate, and a foreign sheet is never clobbered.
		plain = filepath.Join(sdtSourcesDir, slug+"-"+record.SHA256[:8]+sdtMarkdownExt)
		content, exists, err = readSourceSheet(plain)
		if err != nil {
			return "", "", err
		}
		if exists && !sheetPinsSource(content, archivedCorpus, record.SHA256) {
			return plain, "", nil
		}
	}
	if exists {
		action = "linked"
	} else {
		action = statusCreated
		if content, err = newSourceSheetContent(slug, archivedCorpus, record, converted); err != nil {
			return "", "", err
		}
	}
	content = insertFrontmatterScalars(content, keys)
	if action == "created" {
		content += "\n" + sourceSheetBody(archivedCorpus, record, converted)
	}
	if err := os.WriteFile(plain, []byte(content), 0o644); err != nil { //#nosec G306 -- user work file, `context new`'s own mode
		return "", "", err
	}
	return corpusRelative(plain), action, nil
}

// readSourceSheet reads a sheet, reporting absence as (empty, false, nil).
func readSourceSheet(path string) (string, bool, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus path under sources/
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(data), true, nil
}

// sheetPinsSource reports whether a sheet already records this archived source,
// by digest or by capture path — the two keys the archive step owns.
func sheetPinsSource(content, archivedCorpus, sha256 string) bool {
	return contextwiki.FrontmatterField(content, "sha256") == sha256 ||
		contextwiki.FrontmatterField(content, "capture_path") == archivedCorpus
}

// ingestSheetKeys is the key set the archive step owns on a sheet.
func ingestSheetKeys(archivedCorpus string, record ingestArchiveRecord, converted string) [][2]string {
	keys := [][2]string{
		{ctxSourceSheetSHA256, record.SHA256},
		{"byte_count", strconv.FormatInt(record.Bytes, 10)},
		{"captured_at", record.ArchivedAt},
		{"capture_path", archivedCorpus},
		{"checked_at", record.ArchivedAt[:10]},
	}
	if converted != "" {
		keys = append(keys, [][2]string{{"extracted_text", converted}}...)
	}
	if record.License != "" {
		keys = append(keys, [][2]string{{"license", record.License}}...)
	}
	return keys
}

// newSourceSheetContent builds a fresh sheet: the kind's own frontmatter (uid,
// summary, status, created, project) plus the four custody sections. With a
// kept rendering the sheet is `curated` — the source was read and distilled;
// without one it stays at the kind's default until a session judges it.
func newSourceSheetContent(slug, archivedCorpus string, record ingestArchiveRecord, converted string) (string, error) {
	title := strings.TrimSuffix(filepath.Base(archivedCorpus), filepath.Ext(archivedCorpus))
	if converted != "" {
		if data, err := os.ReadFile(filepath.Join(sdtWorkDir, filepath.FromSlash(converted))); err == nil { //#nosec G304 -- converted corpus path
			if t := contextwiki.FrontmatterField(string(data), "title"); t != "" {
				title = t
			}
		}
	}
	summary := "Archived source; the sha256 pin is the custody record of the bytes."
	if converted != "" {
		summary = "Archived source of " + converted + "; the sha256 pin is the custody link to the kept rendering."
	}
	project := ""
	if cfg, err := findProjectConfig(); err == nil && cfg != nil {
		project = cfg.Project
	}
	status := ""
	if converted != "" {
		status = "curated"
	}
	created := time.Now().UTC().Format(time.RFC3339)
	return contextFrontmatter(ctxTypeSource, title, summary, "", project, "", created, "", slug, "", "", "", status, nil, nil, nil), nil
}

// sourceSheetBody is the four-section body of a fresh sheet, with the restore
// recipe in the custody notes: re-convert from the archived source when a kept
// rendering exists, otherwise the archived bytes are the artifact.
func sourceSheetBody(archivedCorpus string, record ingestArchiveRecord, converted string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Why it matters\n\nArchived from `%s` on %s; `sha256` pins the bytes", record.Source, record.ArchivedAt[:10])
	if converted != "" {
		fmt.Fprintf(&b, " and `extract` names the kept rendering the corpus cites")
	}
	b.WriteString(".\n\n")
	b.WriteString("## What was taken\n\nNot recorded by the archive step: the citations on `extract` name their own anchors.\n\n")
	b.WriteString("## What was rejected\n\nNothing recorded here: the archive move is custody, not a reading.\n\n")
	b.WriteString("## Custody notes\n\n")
	fmt.Fprintf(&b, "Created by `sdt context ingest archive`; the pin was checked at %s.\n", record.ArchivedAt[:10])
	if converted != "" {
		fmt.Fprintf(&b, "Restore: `sdt doc2md %s --keep --force` — a re-derivation changes bytes, so the citations on `extract` re-pin (a migration, not a repair).\n", archivedCorpus)
	} else {
		b.WriteString("Restore: none recorded — the archived bytes are the artifact; re-acquiring from the origin produces new bytes and a new pin.\n")
	}
	return b.String()
}

// insertFrontmatterScalars inserts missing `key: value` lines after the
// `summary:` line of a document's frontmatter (after `kind:` when the summary
// is absent), skipping keys it already carries, so linking an existing sheet
// fills only what is missing and never reorders or re-quotes what is there.
func insertFrontmatterScalars(content string, keys [][2]string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return content
	}
	fm, rest := content[:end+4+1], content[end+4+1:]
	present := map[string]bool{}
	for _, line := range strings.Split(fm, "\n") {
		if strings.HasPrefix(line, " ") {
			continue
		}
		if k, _, found := strings.Cut(line, ":"); found {
			present[strings.TrimSpace(k)] = true
		}
	}
	var insert []string
	for _, kv := range keys {
		if present[kv[0]] {
			continue
		}
		present[kv[0]] = true
		insert = append(insert, kv[0]+": "+yamlScalar(kv[1]))
	}
	if len(insert) == 0 {
		return content
	}
	lines := strings.Split(fm, "\n")
	at := 1
	for i := 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "summary:") {
			at = i + 1
			break
		}
		if strings.HasPrefix(lines[i], "kind:") {
			at = i + 1
		}
	}
	merged := make([]string, 0, len(lines)+len(insert))
	merged = append(merged, lines[:at]...)
	merged = append(merged, insert...)
	merged = append(merged, lines[at:]...)
	return strings.Join(merged, "\n") + rest
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func init() {
	contextIngestListCmd.Flags().Bool("write", false, "Write the derived manifest to context/ingestion/manifest.yaml")
	contextIngestArchiveCmd.Flags().Bool("force", false, "Archive even when no converted artifact matches")
	contextIngestArchiveCmd.Flags().String("classification", "", "What the source declares about its use: "+strings.Join(sourceclass.Tiers(), "|"))
	contextIngestArchiveCmd.Flags().String("license", "", "The source's licence (recorded as declared, never interpreted)")
	contextIngestArchiveCmd.Flags().Bool("dry-run", false, "Print the archive plan without moving the file")
	contextIngestPlanCmd.Flags().String("artifact", "", "Converted artifact to measure (optional)")
	contextIngestPlanCmd.Flags().String("inventory", "", "Concept-inventory note to name in the plan (optional)")
	contextIngestPlanCmd.Flags().String("scope", "all", "Bounded units the run will read")
	contextIngestPlanCmd.Flags().String("mode", "merge", "merge|reset")
	contextIngestPlanCmd.Flags().Bool("write", false, "Persist the plan under context/ingestion/runs/")
	contextIngestCmd.AddCommand(contextIngestListCmd)
	contextIngestCmd.AddCommand(contextIngestStatusCmd)
	contextIngestCmd.AddCommand(contextIngestArchiveCmd)
	contextIngestCmd.AddCommand(contextIngestPlanCmd)
	contextCmd.AddCommand(contextIngestCmd)
}
