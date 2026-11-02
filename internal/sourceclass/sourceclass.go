// Package sourceclass carries what a source declares about its own use — a
// classification and a licence — and the rule a page follows when it cites
// several: the strictest wins, and a mix that cannot be flattened is reported
// rather than resolved.
//
// The declaration lives on the archive sidecar (`<basename>.source.yaml`), not on
// the page: a page is a projection of its sources (decision 0035), so the page
// inherits, and a page that declares its own value may only be stricter.
package sourceclass

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// Tiers, weakest first. An empty tier means the source never declared one, which
// is not the same as `public`: silence is not a licence to publish.
const (
	TierPublic       = "public"
	TierInternal     = "internal"
	TierConfidential = "confidential"
)

// SidecarSuffix is the archive provenance file written beside a source.
const SidecarSuffix = ".source.yaml"

// Tiers returns the closed classification vocabulary, weakest first.
func Tiers() []string { return []string{TierPublic, TierInternal, TierConfidential} }

// Rank orders a tier; -1 when the value is not in the vocabulary.
func Rank(tier string) int {
	for i, t := range Tiers() {
		if t == strings.TrimSpace(tier) {
			return i
		}
	}
	return -1
}

// Known reports whether a value is in the vocabulary.
func Known(tier string) bool { return Rank(tier) >= 0 }

// Meta is what one source declares.
type Meta struct {
	Classification string `yaml:"classification,omitempty"`
	License        string `yaml:"license,omitempty"`
}

// Page is the inherited view of a set of sources: the strictest classification,
// the licences that agree, and the reasons a reader has to look at the page.
type Page struct {
	Classification string
	License        string
	// Findings name what could not be flattened: a mixed classification, or two
	// licences that do not agree. Empty means the page inherits cleanly.
	Findings []string
}

// ForSources resolves the inheritance for a page's `sources` entries. A missing
// sidecar is an undeclared source, not an error: most sources predate the field.
func ForSources(sources []string) Page {
	var page Page
	strictest := -1
	classes := map[string]bool{}
	licenses := map[string]bool{}
	for _, src := range sources {
		meta, ok := readSidecar(src)
		if !ok {
			continue // an unarchived or undeclared source is not a page defect
		}
		if class := strings.TrimSpace(meta.Classification); class != "" {
			classes[class] = true
			if rank := Rank(class); rank > strictest {
				strictest = rank
				page.Classification = class
			}
		}
		if lic := strings.TrimSpace(meta.License); lic != "" {
			licenses[lic] = true
		}
	}
	if len(classes) > 1 {
		page.Findings = append(page.Findings, "sources declare different classifications ("+joinKeys(classes)+"); the page inherits the strictest and must say so")
	}
	switch len(licenses) {
	case 0:
	case 1:
		for lic := range licenses {
			page.License = lic
		}
	default:
		page.Findings = append(page.Findings, "sources carry different licences ("+joinKeys(licenses)+"); a page cannot inherit one of them silently")
	}
	return page
}

// CheckPage compares what a page declares with what it inherits. A page may be
// stricter than its sources and may not be weaker; a page that inherits a mixed
// set must state the strictest one explicitly.
func CheckPage(declaredClassification, declaredLicense string, inherited Page) []string {
	var findings []string
	declared := strings.TrimSpace(declaredClassification)
	if declared != "" && !Known(declared) {
		findings = append(findings, fmt.Sprintf("classification %q is not one of %s", declared, strings.Join(Tiers(), ", ")))
	}
	if inherited.Classification != "" {
		switch {
		case declared == "" && len(inherited.Findings) > 0:
			findings = append(findings, "sources mix classifications; state the strictest one on the page")
		case declared != "" && Rank(declared) < Rank(inherited.Classification):
			findings = append(findings, fmt.Sprintf("classification %q is weaker than the %q its sources declare", declared, inherited.Classification))
		}
	}
	if lic := strings.TrimSpace(declaredLicense); lic != "" && inherited.License != "" && lic != inherited.License {
		findings = append(findings, fmt.Sprintf("licence %q disagrees with the %q its sources declare", lic, inherited.License))
	}
	return append(findings, inherited.Findings...)
}

// readSidecar reads `<source>.source.yaml` relative to the project root. A source
// is stored corpus-relative (`refs/...`), so the sidecar sits beside it under
// `context/`.
func readSidecar(source string) (Meta, bool) {
	rel := strings.TrimSpace(source)
	if rel == "" {
		return Meta{}, false
	}
	path := filepath.Join("context", rel) + SidecarSuffix
	data, err := os.ReadFile(path) //#nosec G304 -- corpus path built from a frontmatter entry
	if err != nil {
		return Meta{}, false
	}
	var meta Meta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return Meta{}, false
	}
	return meta, true
}

func joinKeys(set map[string]bool) string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	// Stable order keeps the finding's text reproducible.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return strings.Join(keys, ", ")
}
