package cmd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/distill"
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
	Source     string `yaml:"source"`
	Bytes      int64  `yaml:"bytes"`
	SHA256     string `yaml:"sha256"`
	Converted  string `yaml:"converted,omitempty"`
	ArchivedAt string `yaml:"archived_at"`
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
binary. --dry-run prints the plan and writes nothing.`,
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
		if converted == "" && !getBoolFlag(cmd, "force", false) {
			exitWithError(cmd, fmt.Errorf("%s has no matching converted artifact; archive it with --force only after the run is dispositioned", corpusRelative(path)))
			return
		}
		dest := filepath.Join(sdtRefsDir, filepath.Base(path))
		if _, statErr := os.Stat(dest); statErr == nil {
			exitWithError(cmd, fmt.Errorf("archive destination %s already exists", corpusRelative(dest)))
			return
		}
		if getBoolFlag(cmd, "dry-run", false) {
			outputString(cmd, fmt.Sprintf("would archive: %s -> %s (sha256 %s)\n", corpusRelative(path), corpusRelative(dest), sha[:12]))
			return
		}
		if err := os.Rename(path, dest); err != nil {
			exitWithError(cmd, fmt.Errorf("archive %s: %w", corpusRelative(path), err))
			return
		}
		record := ingestArchiveRecord{
			Source:     corpusRelative(path),
			Bytes:      bytes,
			SHA256:     sha,
			Converted:  converted,
			ArchivedAt: time.Now().UTC().Format(time.RFC3339),
		}
		out, merr := yaml.Marshal(record)
		if merr != nil {
			exitWithError(cmd, merr)
			return
		}
		sidecar := dest + ingestSidecarSuffix
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
		route, headings, lines, units := "unknown", 0, 0, 0
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
	h := sha256.New()
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

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func init() {
	contextIngestListCmd.Flags().Bool("write", false, "Write the derived manifest to context/ingestion/manifest.yaml")
	contextIngestArchiveCmd.Flags().Bool("force", false, "Archive even when no converted artifact matches")
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
