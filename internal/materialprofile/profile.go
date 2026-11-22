package materialprofile

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

var profileIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var profileRevisionPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

// Profile is a reusable description of a material's intended reader. It must
// never contain an individual's identity, learner state, or progress.
type Profile struct {
	SchemaVersion          string   `yaml:"schema_version" json:"schema_version"`
	ID                     string   `yaml:"id" json:"id"`
	Revision               string   `yaml:"revision" json:"revision"`
	Title                  string   `yaml:"title" json:"title"`
	PriorKnowledge         []string `yaml:"prior_knowledge" json:"prior_knowledge"`
	TerminologyFamiliarity []string `yaml:"terminology_familiarity" json:"terminology_familiarity"`
	Register               string   `yaml:"register" json:"register"`
	ExampleContexts        []string `yaml:"example_contexts" json:"example_contexts"`
	AccessibilityNeeds     []string `yaml:"accessibility_needs" json:"accessibility_needs"`
}

// Decode validates a profile document and requires its declared identity to
// match the path identity supplied by the caller.
func Decode(data []byte, expectedID, expectedRevision string) (Profile, error) {
	var profile Profile
	decoder := yaml.NewDecoder(bytes.NewReader(data), yaml.Strict())
	if err := decoder.Decode(&profile); err != nil {
		return Profile{}, fmt.Errorf("decode recipient profile: %w", err)
	}
	if err := profile.Validate(expectedID, expectedRevision); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func Encode(profile Profile) ([]byte, error) {
	if err := profile.Validate("", ""); err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("encode recipient profile: %w", err)
	}
	return data, nil
}

func (p Profile) Validate(expectedID, expectedRevision string) error {
	if p.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version %q is unsupported (want %q)", p.SchemaVersion, SchemaVersion)
	}
	if !profileIDPattern.MatchString(p.ID) {
		return fmt.Errorf("profile id %q must be a lowercase kebab-case identifier", p.ID)
	}
	if !profileRevisionPattern.MatchString(p.Revision) {
		return fmt.Errorf("profile revision %q must be a positive integer", p.Revision)
	}
	if expectedID != "" && p.ID != expectedID {
		return fmt.Errorf("profile id %q does not match requested id %q", p.ID, expectedID)
	}
	if expectedRevision != "" && p.Revision != expectedRevision {
		return fmt.Errorf("profile revision %q does not match requested revision %q", p.Revision, expectedRevision)
	}
	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("profile title is required")
	}
	if p.PriorKnowledge == nil || p.TerminologyFamiliarity == nil || strings.TrimSpace(p.Register) == "" || p.ExampleContexts == nil || p.AccessibilityNeeds == nil {
		return fmt.Errorf("profile must explicitly declare prior_knowledge, terminology_familiarity, register, example_contexts, and accessibility_needs; use empty lists when none apply")
	}
	for field, values := range map[string][]string{
		"prior_knowledge":         p.PriorKnowledge,
		"terminology_familiarity": p.TerminologyFamiliarity,
		"example_contexts":        p.ExampleContexts,
		"accessibility_needs":     p.AccessibilityNeeds,
	} {
		for i, value := range values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s[%d] must not be blank", field, i)
			}
		}
	}
	return nil
}

// Read loads one immutable revision beneath context/learning/profiles.
func Read(projectRoot, id, revision string) (Profile, error) {
	if !profileIDPattern.MatchString(id) || !profileRevisionPattern.MatchString(revision) {
		return Profile{}, fmt.Errorf("invalid recipient profile path identity %q revision %q", id, revision)
	}
	project, err := os.OpenRoot(projectRoot)
	if err != nil {
		return Profile{}, err
	}
	defer project.Close() //nolint:errcheck // read-only handle cleanup
	root, err := project.OpenRoot(filepath.Join("context", "learning", "profiles"))
	if err != nil {
		if os.IsNotExist(err) {
			return Profile{}, fmt.Errorf("recipient profile %s@%s not found", id, revision)
		}
		return Profile{}, err
	}
	defer root.Close() //nolint:errcheck // read-only handle cleanup

	path := filepath.Join(id, revision+".yaml")
	data, err := root.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Profile{}, fmt.Errorf("recipient profile %s@%s not found", id, revision)
		}
		return Profile{}, fmt.Errorf("read recipient profile %s@%s: %w", id, revision, err)
	}
	return Decode(data, id, revision)
}
