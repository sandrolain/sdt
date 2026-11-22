// Package materialbrief validates immutable composition briefs for learning materials.
package materialbrief

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

const SchemaVersion = "1"

var revisionPattern = regexp.MustCompile(`^[1-9][0-9]*$`)
var idSegmentPattern = regexp.MustCompile(`^[a-z0-9]+(?:[.-][a-z0-9]+)*$`)

type Brief struct {
	SchemaVersion string         `yaml:"schema_version" json:"schema_version"`
	ID            string         `yaml:"id" json:"id"`
	Revision      string         `yaml:"revision" json:"revision"`
	Title         string         `yaml:"title" json:"title"`
	Profile       *ProfileRef    `yaml:"profile,omitempty" json:"profile,omitempty"`
	Outcome       string         `yaml:"outcome" json:"outcome"`
	ContentScope  []string       `yaml:"content_scope" json:"content_scope"`
	Forms         []string       `yaml:"forms" json:"forms"`
	Length        Length         `yaml:"length" json:"length"`
	Sources       []Source       `yaml:"sources" json:"sources"`
	Overrides     []Override     `yaml:"overrides" json:"overrides"`
	Examples      []TeachingCase `yaml:"teaching_examples" json:"teaching_examples"`
}

type ProfileRef struct {
	ID       string `yaml:"id" json:"id"`
	Revision string `yaml:"revision" json:"revision"`
}

type Length struct {
	MinWords int `yaml:"min_words" json:"min_words"`
	MaxWords int `yaml:"max_words" json:"max_words"`
}

type Source struct {
	Kind    string `yaml:"kind" json:"kind"`
	Ref     string `yaml:"ref" json:"ref"`
	Version string `yaml:"version" json:"version"`
}

type Override struct {
	Field  string `yaml:"field" json:"field"`
	Value  string `yaml:"value" json:"value"`
	Reason string `yaml:"reason" json:"reason"`
}

type TeachingCase struct {
	Description string `yaml:"description" json:"description"`
	FaithfulTo  string `yaml:"faithful_to" json:"faithful_to"`
}

var supportedForms = map[string]bool{
	"conceptual-explanation": true,
	"procedural-guide":       true,
	"decision-case":          true,
	"operational-reference":  true,
}

func Decode(data []byte, expectedID, expectedRevision string) (Brief, error) {
	var brief Brief
	decoder := yaml.NewDecoder(bytes.NewReader(data), yaml.Strict())
	if err := decoder.Decode(&brief); err != nil {
		return Brief{}, fmt.Errorf("decode material brief: %w", err)
	}
	if err := brief.Validate(expectedID, expectedRevision); err != nil {
		return Brief{}, err
	}
	return brief, nil
}

func Encode(brief Brief) ([]byte, error) {
	if err := brief.Validate("", ""); err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(brief)
	if err != nil {
		return nil, fmt.Errorf("encode material brief: %w", err)
	}
	return data, nil
}

func (b Brief) Validate(expectedID, expectedRevision string) error {
	if b.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version %q is unsupported (want %q)", b.SchemaVersion, SchemaVersion)
	}
	if !validMaterialID(b.ID) {
		return fmt.Errorf("material id %q is invalid", b.ID)
	}
	if !revisionPattern.MatchString(b.Revision) {
		return fmt.Errorf("brief revision %q must be a positive integer", b.Revision)
	}
	if expectedID != "" && b.ID != expectedID {
		return fmt.Errorf("brief id %q does not match requested id %q", b.ID, expectedID)
	}
	if expectedRevision != "" && b.Revision != expectedRevision {
		return fmt.Errorf("brief revision %q does not match requested revision %q", b.Revision, expectedRevision)
	}
	if strings.TrimSpace(b.Title) == "" || strings.TrimSpace(b.Outcome) == "" {
		return fmt.Errorf("brief title and outcome are required")
	}
	if len(b.ContentScope) == 0 || len(b.Forms) == 0 || len(b.Sources) == 0 {
		return fmt.Errorf("brief requires content_scope, forms, and versioned sources")
	}
	if err := validateScope(b.ContentScope); err != nil {
		return err
	}
	if err := validateForms(b.Forms); err != nil {
		return err
	}
	if err := validateLength(b.Length); err != nil {
		return err
	}
	if err := validateSources(b.Sources); err != nil {
		return err
	}
	if err := validateProfileRef(b.Profile); err != nil {
		return err
	}
	if err := validateOverrides(b.Overrides); err != nil {
		return err
	}
	return validateTeachingExamples(b.Examples)
}

func validateScope(scopeItems []string) error {
	for i, scope := range scopeItems {
		if strings.TrimSpace(scope) == "" {
			return fmt.Errorf("content_scope[%d] must not be blank", i)
		}
	}
	return nil
}

func validateForms(forms []string) error {
	seenForms := map[string]bool{}
	for _, form := range forms {
		if !supportedForms[form] {
			return fmt.Errorf("unsupported instructional form %q", form)
		}
		if seenForms[form] {
			return fmt.Errorf("duplicate instructional form %q", form)
		}
		seenForms[form] = true
	}
	return nil
}

func validateLength(length Length) error {
	if length.MinWords <= 0 || length.MaxWords <= 0 || length.MaxWords < length.MinWords {
		return fmt.Errorf("brief length requires positive min_words and max_words, with max_words >= min_words")
	}
	return nil
}

func validateSources(sources []Source) error {
	seenSources := map[string]string{}
	for i, source := range sources {
		if source.Kind != "claim" && source.Kind != "objective" && source.Kind != "case" && source.Kind != "lesson" {
			return fmt.Errorf("sources[%d] has unsupported kind %q", i, source.Kind)
		}
		if strings.TrimSpace(source.Ref) == "" || strings.TrimSpace(source.Version) == "" {
			return fmt.Errorf("sources[%d] requires ref and version", i)
		}
		key := source.Kind + "\x00" + source.Ref
		if previous, exists := seenSources[key]; exists && previous != source.Version {
			return fmt.Errorf("source %s %q has conflicting versions %q and %q", source.Kind, source.Ref, previous, source.Version)
		}
		seenSources[key] = source.Version
	}
	return nil
}

func validateProfileRef(profile *ProfileRef) error {
	if profile != nil {
		if !profileIDPattern.MatchString(profile.ID) || !revisionPattern.MatchString(profile.Revision) {
			return fmt.Errorf("brief profile reference must name a valid profile id and positive revision")
		}
	}
	return nil
}

func validateOverrides(overrides []Override) error {
	for i, override := range overrides {
		if strings.TrimSpace(override.Field) == "" || strings.TrimSpace(override.Value) == "" || strings.TrimSpace(override.Reason) == "" {
			return fmt.Errorf("overrides[%d] requires field, value, and reason", i)
		}
	}
	return nil
}

func validateTeachingExamples(examples []TeachingCase) error {
	for i, example := range examples {
		if strings.TrimSpace(example.Description) == "" || strings.TrimSpace(example.FaithfulTo) == "" {
			return fmt.Errorf("teaching_examples[%d] must state description and faithful_to", i)
		}
	}
	return nil
}

func Read(projectRoot, materialID, revision string) (Brief, error) {
	if !validMaterialID(materialID) || !revisionPattern.MatchString(revision) {
		return Brief{}, fmt.Errorf("invalid material brief path identity %q revision %q", materialID, revision)
	}
	project, err := os.OpenRoot(projectRoot)
	if err != nil {
		return Brief{}, err
	}
	defer project.Close() //nolint:errcheck // read-only handle cleanup
	root, err := project.OpenRoot(filepath.Join("context", "learning", "briefs"))
	if err != nil {
		if os.IsNotExist(err) {
			return Brief{}, fmt.Errorf("material brief %s@%s not found", materialID, revision)
		}
		return Brief{}, err
	}
	defer root.Close() //nolint:errcheck // read-only handle cleanup
	data, err := root.ReadFile(filepath.Join(filepath.FromSlash(materialID), revision+".yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return Brief{}, fmt.Errorf("material brief %s@%s not found", materialID, revision)
		}
		return Brief{}, fmt.Errorf("read material brief %s@%s: %w", materialID, revision, err)
	}
	return Decode(data, materialID, revision)
}

func validMaterialID(value string) bool {
	if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." || !idSegmentPattern.MatchString(segment) {
			return false
		}
	}
	return true
}

func ValidMaterialID(value string) bool { return validMaterialID(value) }

var profileIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
