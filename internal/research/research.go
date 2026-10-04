// Package research holds the pure run model behind the `sdt research` command
// family: an auditable YAML run manifest with a stable on-disk layout, source
// status transitions and a resume checkpoint. It has no cobra dependency, so
// the model can be tested and reused without the CLI.
package research

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"uuid"

	"github.com/goccy/go-yaml"
)

// RunDirName is the transient directory, relative to the project root, that
// holds every research run.
const RunDirName = "context/tmp/research"

// ManifestName is the machine-readable run manifest inside a run directory.
const ManifestName = "manifest.yaml"

// ProvenanceName is the human-readable sidecar beside the manifest.
const ProvenanceName = "provenance.md"

// RawDirName holds the captured payloads of a run.
const RawDirName = "raw"

// objectiveSlugMax bounds a run objective slug, so a noisy query cannot produce
// an over-long archive directory name.
const objectiveSlugMax = 48

// Source statuses. A source advances discovered → fetched → parsed → verified,
// or ends rejected.
const (
	StatusDiscovered = "discovered"
	StatusFetched    = "fetched"
	StatusParsed     = "parsed"
	StatusVerified   = "verified"
	StatusRejected   = "rejected"
)

// Source is one item of the run: a URL discovered by a search provider or named
// on the command line, plus the provenance of its acquisition.
type Source struct {
	CanonicalURL string `json:"canonical_url" yaml:"canonical_url"`
	Title        string `json:"title,omitempty" yaml:"title,omitempty"`
	Snippet      string `json:"snippet,omitempty" yaml:"snippet,omitempty"`
	Author       string `json:"author,omitempty" yaml:"author,omitempty"`
	Date         string `json:"date,omitempty" yaml:"date,omitempty"`
	License      string `json:"license,omitempty" yaml:"license,omitempty"`
	ContentType  string `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	Bytes        int64  `json:"bytes,omitempty" yaml:"bytes,omitempty"`
	SHA256       string `json:"sha256,omitempty" yaml:"sha256,omitempty"`
	RawPath      string `json:"raw_path,omitempty" yaml:"raw_path,omitempty"`
	Status       string `json:"status" yaml:"status"`
	Error        string `json:"error,omitempty" yaml:"error,omitempty"`
	Retries      int    `json:"retries,omitempty" yaml:"retries,omitempty"`
}

// Budget records the limits declared for a run and what was consumed.
type Budget struct {
	MaxResults  int `json:"max_results,omitempty" yaml:"max_results,omitempty"`
	MaxCredits  int `json:"max_credits,omitempty" yaml:"max_credits,omitempty"`
	CreditsUsed int `json:"credits_used,omitempty" yaml:"credits_used,omitempty"`
	Results     int `json:"results,omitempty" yaml:"results,omitempty"`
}

// Checkpoint records the last completed operation, so an interrupted run can be
// resumed without repeating work.
type Checkpoint struct {
	Operation string `json:"operation,omitempty" yaml:"operation,omitempty"`
	At        string `json:"at,omitempty" yaml:"at,omitempty"`
}

// Run is the whole manifest.
type Run struct {
	RunID      string     `json:"run_id" yaml:"run_id"`
	Query      string     `json:"query" yaml:"query"`
	Scope      string     `json:"scope,omitempty" yaml:"scope,omitempty"`
	Objective  string     `json:"objective,omitempty" yaml:"objective,omitempty"`
	ArchiveDir string     `json:"archive_dir,omitempty" yaml:"archive_dir,omitempty"`
	Created    string     `json:"created" yaml:"created"`
	Updated    string     `json:"updated" yaml:"updated"`
	Tool       string     `json:"tool,omitempty" yaml:"tool,omitempty"`
	Config     string     `json:"config,omitempty" yaml:"config,omitempty"`
	Budget     Budget     `json:"budget,omitempty" yaml:"budget,omitempty"`
	Checkpoint Checkpoint `json:"checkpoint,omitempty" yaml:"checkpoint,omitempty"`
	Sources    []Source   `json:"sources" yaml:"sources"`
}

// RunDir returns the run directory under a project root.
func RunDir(root, runID string) string {
	return filepath.Join(root, RunDirName, runID)
}

// ManifestPath returns the manifest path under a project root.
func ManifestPath(root, runID string) string {
	return filepath.Join(RunDir(root, runID), ManifestName)
}

// NewRun builds a fresh run with a time-ordered id. objective is the explicit
// grouping key; when empty it is derived from the query (see ObjectiveSlug).
func NewRun(query, scope, objective string, now time.Time) *Run {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stamp := now.UTC().Format(time.RFC3339)
	if strings.TrimSpace(objective) == "" {
		objective = ObjectiveSlug(&Run{Query: query})
	}
	return &Run{
		RunID:     uuid.NewV7().String(),
		Query:     query,
		Scope:     scope,
		Objective: objective,
		Created:   stamp,
		Updated:   stamp,
		Sources:   []Source{},
	}
}

// ObjectiveSlug returns the slug used for the run's archive directory: the
// run's explicit objective when set, otherwise a kebab-case slug derived from
// the query, otherwise "research". The result is always a valid, non-empty
// kebab-case slug bounded to objectiveSlugMax characters, so it can never yield
// an empty path segment.
func ObjectiveSlug(r *Run) string {
	if r == nil {
		return "research"
	}
	s := slugifySlug(r.Objective)
	if s == "" {
		s = slugifySlug(r.Query)
	}
	if len(s) > objectiveSlugMax {
		s = strings.Trim(s[:objectiveSlugMax], "-")
	}
	if s == "" {
		return "research"
	}
	return s
}

// Save writes the manifest and the provenance sidecar, creating the run
// directory (and raw/ inside it). The manifest is written last, so a partial
// run directory never carries a manifest that claims success.
func (r *Run) Save(root string) error {
	dir := RunDir(root, r.RunID)
	if err := os.MkdirAll(filepath.Join(dir, RawDirName), 0o750); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}
	r.Updated = time.Now().UTC().Format(time.RFC3339)

	if err := os.WriteFile(filepath.Join(dir, ProvenanceName), []byte(r.Provenance()), 0o600); err != nil {
		return fmt.Errorf("write provenance: %w", err)
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestName), data, 0o600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// Load reads a run manifest from a project root.
func Load(root, runID string) (*Run, error) {
	//#nosec G304 -- run id resolved within the project tmp directory
	data, err := os.ReadFile(ManifestPath(root, runID))
	if err != nil {
		return nil, err
	}
	var r Run
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if r.Sources == nil {
		r.Sources = []Source{}
	}
	return &r, nil
}

// FindLatest returns the most recently created run id under a root, or "" when
// there is no run.
func FindLatest(root string) (string, error) {
	base := filepath.Join(root, RunDirName)
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	latest, latestTime := "", time.Time{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latest, latestTime = e.Name(), info.ModTime()
		}
	}
	return latest, nil
}

// AddSource inserts s if its canonical URL is not already present (dedup by
// canonical URL). It reports whether the source was added.
func (r *Run) AddSource(s Source) bool {
	key := CanonicalURL(s.CanonicalURL)
	for i := range r.Sources {
		if CanonicalURL(r.Sources[i].CanonicalURL) == key {
			return false
		}
	}
	if s.Status == "" {
		s.Status = StatusDiscovered
	}
	s.CanonicalURL = key
	r.Sources = append(r.Sources, s)
	return true
}

// Source returns a pointer to the source with the given canonical URL, or nil.
func (r *Run) Source(canonical string) *Source {
	key := CanonicalURL(canonical)
	for i := range r.Sources {
		if CanonicalURL(r.Sources[i].CanonicalURL) == key {
			return &r.Sources[i]
		}
	}
	return nil
}

// MarkCheckpoint records the last completed operation.
func (r *Run) MarkCheckpoint(operation string) {
	r.Checkpoint = Checkpoint{Operation: operation, At: time.Now().UTC().Format(time.RFC3339)}
}

// RawRelPath returns the run-relative raw path for a source slug, as recorded in
// the manifest (relative so a moved run keeps resolving).
func RawRelPath(slug string) string {
	return filepath.ToSlash(filepath.Join(RawDirName, slug+".md"))
}

// RecordFetch records the successful acquisition of a source: it stores the raw
// payload under raw/, then updates (or adds) the manifest entry with the
// provenance and the fetched status. The hash covers the exact bytes written.
func (r *Run) RecordFetch(rawDir string, src Source, payload []byte) error {
	if src.CanonicalURL == "" {
		return errors.New("source has no canonical URL")
	}
	src.CanonicalURL = CanonicalURL(src.CanonicalURL)
	if err := os.MkdirAll(rawDir, 0o750); err != nil {
		return fmt.Errorf("create raw dir: %w", err)
	}
	slug := SourceSlug(src.CanonicalURL)
	rel := RawRelPath(slug)
	abs := filepath.Join(filepath.Dir(rawDir), rel)
	if err := os.WriteFile(abs, payload, 0o600); err != nil {
		return fmt.Errorf("write raw payload: %w", err)
	}

	sum := sha256.Sum256(payload)
	src.SHA256 = fmt.Sprintf("%x", sum[:])
	src.Bytes = int64(len(payload))
	src.RawPath = rel
	src.Status = StatusFetched
	src.Error = ""

	if existing := r.Source(src.CanonicalURL); existing != nil {
		*existing = src
		return nil
	}
	r.Sources = append(r.Sources, src)
	return nil
}

// RecordFailure records a failed acquisition on the source, preserving the
// status as discovered (so a resume retries it) and incrementing the retry
// count and the error.
func (r *Run) RecordFailure(canonical string, err error) {
	key := CanonicalURL(canonical)
	if key == "" {
		return
	}
	if existing := r.Source(key); existing != nil {
		existing.Retries++
		existing.Error = err.Error()
		if existing.Status == "" {
			existing.Status = StatusDiscovered
		}
		return
	}
	r.Sources = append(r.Sources, Source{
		CanonicalURL: key,
		Status:       StatusDiscovered,
		Retries:      1,
		Error:        err.Error(),
	})
}

// SourceSlug derives a stable, filesystem-safe slug for a source URL.
func SourceSlug(rawURL string) string {
	u := CanonicalURL(rawURL)
	sum := sha256.Sum256([]byte(u))
	return fmt.Sprintf("%s-%s", urlSlug(u), fmt.Sprintf("%x", sum[:])[:8])
}

// urlSlug derives a filesystem-safe slug from a URL's last path segment, or its
// host when the path is empty. It never returns an empty string ("result" is the
// last resort), so it is safe inside a path segment.
func urlSlug(rawURL string) string {
	u := CanonicalURL(rawURL)
	// Prefer the last path segment; fall back to the host.
	trimmed := strings.TrimPrefix(u, "http://")
	trimmed = strings.TrimPrefix(trimmed, "https://")
	host := trimmed
	path := ""
	if i := strings.IndexByte(trimmed, '/'); i >= 0 {
		host = trimmed[:i]
		path = trimmed[i+1:]
	}
	base := path
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".html")
	base = strings.TrimSuffix(base, ".htm")
	if base == "" {
		base = host
	}
	slug := slugifySlug(base)
	if slug == "" {
		slug = slugifySlug(host)
	}
	if slug == "" {
		slug = "result"
	}
	return slug
}

func slugifySlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// CountByStatus returns how many sources are in the given status.
func (r *Run) CountByStatus(status string) int {
	n := 0
	for _, s := range r.Sources {
		if s.Status == status {
			n++
		}
	}
	return n
}

// PendingFetch returns the canonical URLs that still need acquisition: sources
// recorded as discovered (never fetched, or a failed fetch to retry). Sources
// already fetched/parsed/verified are skipped.
func (r *Run) PendingFetch() []string {
	var out []string
	for _, s := range r.Sources {
		if s.Status == StatusDiscovered {
			out = append(out, s.CanonicalURL)
		}
	}
	return out
}

// PendingVerify returns how many sources can still be verified (fetched or
// parsed but not yet checked).
func (r *Run) PendingVerify() int {
	n := 0
	for _, s := range r.Sources {
		if s.Status == StatusFetched || s.Status == StatusParsed {
			n++
		}
	}
	return n
}

// Provenance renders the human-readable sidecar: the run identity, its budget
// and checkpoint, and one line per source with status and hash.
func (r *Run) Provenance() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Research run %s\n\n", r.RunID)
	fmt.Fprintf(&b, "- Query: %s\n", r.Query)
	if r.Scope != "" {
		fmt.Fprintf(&b, "- Scope: %s\n", r.Scope)
	}
	fmt.Fprintf(&b, "- Created: %s\n", r.Created)
	fmt.Fprintf(&b, "- Updated: %s\n", r.Updated)
	if len(r.Sources) > 0 {
		fmt.Fprintf(&b, "\n## Sources\n\n")
		for _, s := range r.Sources {
			fmt.Fprintf(&b, "- [%s] %s", s.Status, s.CanonicalURL)
			if s.SHA256 != "" {
				fmt.Fprintf(&b, " (%s)", s.SHA256[:min(12, len(s.SHA256))])
			}
			if s.Error != "" {
				fmt.Fprintf(&b, " — %s", s.Error)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// CanonicalURL normalizes a URL for dedup: it trims surrounding spaces and a
// trailing slash. It is deliberately conservative — it never rewrites the URL
// beyond safe normalization.
func CanonicalURL(u string) string {
	return strings.TrimSuffix(strings.TrimSpace(u), "/")
}

// HashFile returns the SHA-256 of a file, reading it in one pass.
func HashFile(path string) (string, int64, error) {
	//#nosec G304 -- path recorded in the run manifest
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close() //nolint:errcheck // read-only handle
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), size, nil
}

// ErrNotFound is returned when a run id does not resolve.
var ErrNotFound = errors.New("research run not found")
