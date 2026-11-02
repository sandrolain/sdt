// Package release is the versioned exchange contract between this CLI and a
// consumer of the corpus — a portal, a course builder, an exporter.
//
// A release is a derived, immutable dump (decision 0035): pages with their
// claims, the lesson items that teach them, the sequences a procedure declares,
// and the reuse metadata and classification a page inherits. It is produced from
// the shared builder (`internal/contextwiki`) and never parsed a second time, so
// the dump and the graph cannot disagree about what a page carries.
//
// The package owns the schema version and the Go mirror of it: the schema
// document is embedded, the mirror decodes strictly, and a release that names a
// version this build does not know is refused rather than read optimistically.
package release

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Version is the schema version this build writes and reads.
const Version = "1.0"

// Schema is the versioned JSON Schema of a release, for a consumer that does not
// link this package.
//
//go:embed schema.json
var Schema []byte

// Release is one dump of the corpus.
type Release struct {
	SchemaVersion string `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
	Pages         []Page `json:"pages"`
}

// Page is one wiki page with what a consumer needs to render and grade it.
type Page struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Type    string     `json:"type"`
	Status  string     `json:"status"`
	Summary string     `json:"summary"`
	Tags    []string   `json:"tags,omitempty"`
	Sources []string   `json:"sources,omitempty"`
	Claims  []Claim    `json:"claims"`
	Items   []Item     `json:"items,omitempty"`
	Wires   []Relation `json:"relations,omitempty"`
	// Sequence is the ordered list of claim ids a procedure declares. Absent
	// until a page needs one (decision 0035, point 6).
	Sequence []string `json:"sequence,omitempty"`
	// Classification and License are inherited from the page's sources, the
	// strictest winning; empty means the source never declared one.
	Classification string `json:"classification,omitempty"`
	License        string `json:"license,omitempty"`
	// Reuse is the metadata a catalog filters on. Optional: a page may declare
	// none.
	Reuse *Reuse `json:"reuse,omitempty"`
}

// Claim is one statement with the identity a consumer cites and the digest of the
// text it had when the item was written against it.
type Claim struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Digest string `json:"digest"`
	Text   string `json:"text"`
}

// Item is one assessment item from the page's lesson.
type Item struct {
	ID         string `json:"id"`
	Objective  int    `json:"objective"`
	Type       string `json:"type"`
	Prompt     string `json:"prompt"`
	Expected   string `json:"expected"`
	Rubric     string `json:"rubric"`
	Distractor string `json:"distractor"`
	Claim      string `json:"claim"`
	Grading    string `json:"grading,omitempty"`
	Provenance string `json:"provenance,omitempty"`
	Frozen     string `json:"frozen,omitempty"`
	// Hash is the claim digest the item was written against; absent means the
	// item never pinned one, which a consumer reports as unverified.
	Hash string `json:"hash,omitempty"`
}

// Relation is one typed edge of the knowledge graph.
type Relation struct {
	Verb   string `json:"verb"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
}

// Reuse is the metadata a catalog filters on, orthogonal to the corpus graph.
type Reuse struct {
	Audience string `json:"audience,omitempty"`
	Level    string `json:"level,omitempty"`
	Diataxis string `json:"diataxis,omitempty"`
}

// Validate decodes and checks a release document: the version this build knows,
// the fields a consumer cannot work without, and no field the schema does not
// define. It is the check a valid example passes and an invalid one fails.
func Validate(document []byte) (Release, error) {
	var rel Release
	dec := json.NewDecoder(bytes.NewReader(document))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rel); err != nil {
		return rel, fmt.Errorf("release: %w", err)
	}
	if rel.SchemaVersion != Version {
		return rel, fmt.Errorf("release: schema_version %q is not %q", rel.SchemaVersion, Version)
	}
	if len(rel.Pages) == 0 {
		return rel, errors.New("release: no pages")
	}
	for i, p := range rel.Pages {
		if err := p.validate(); err != nil {
			return rel, fmt.Errorf("release: page %d: %w", i+1, err)
		}
	}
	return rel, nil
}

// validate checks the fields a consumer cannot do without. An empty page is a
// valid page of a release (a stub), but a claim without an id or a digest is not
// a claim, and an item that names neither a claim id nor a prompt is not an item.
func (p Page) validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("missing id")
	}
	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("%s: missing title", p.ID)
	}
	for _, c := range p.Claims {
		if strings.TrimSpace(c.ID) == "" {
			return fmt.Errorf("%s: a claim has no id", p.ID)
		}
		if strings.TrimSpace(c.Digest) == "" {
			return fmt.Errorf("%s: claim %s has no digest", p.ID, c.ID)
		}
		if !strings.Contains(p.ID, "/") && c.Number < 1 {
			return fmt.Errorf("%s: claim %s has no anchor number", p.ID, c.ID)
		}
	}
	for _, it := range p.Items {
		if strings.TrimSpace(it.ID) == "" {
			return fmt.Errorf("%s: an item has no id", p.ID)
		}
		if !strings.Contains(it.Claim, "#") {
			return fmt.Errorf("%s: item %s does not name a claim id", p.ID, it.ID)
		}
	}
	for _, seq := range p.Sequence {
		found := false
		for _, c := range p.Claims {
			if c.ID == seq {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s: sequence names %s, which the page does not carry", p.ID, seq)
		}
	}
	return nil
}
