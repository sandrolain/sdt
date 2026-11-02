package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestValidExampleIsAccepted is half of the contract's acceptance: the shipped
// example decodes into the Go mirror, under the version this build knows.
func TestValidExampleIsAccepted(t *testing.T) {
	rel, err := Validate(readTestFile(t, "valid.json"))
	if err != nil {
		t.Fatalf("the valid example was refused: %v", err)
	}
	if rel.SchemaVersion != Version || len(rel.Pages) != 1 {
		t.Fatalf("decoded release = %+v", rel)
	}
	page := rel.Pages[0]
	if len(page.Claims) != 2 || len(page.Items) != 1 || len(page.Sequence) != 2 {
		t.Fatalf("valid example lost content: %+v", page)
	}
	if page.Items[0].Hash != page.Claims[0].Digest {
		t.Errorf("the item's hash must be the digest of the claim it cites")
	}
}

// TestInvalidExampleIsRefused is the other half: a claim without a digest is not
// a claim, because no consumer can tell which version of the text it pins.
func TestInvalidExampleIsRefused(t *testing.T) {
	_, err := Validate(readTestFile(t, "invalid.json"))
	if err == nil {
		t.Fatal("a claim without a digest was accepted")
	}
	if !strings.Contains(err.Error(), "no digest") {
		t.Errorf("unexpected refusal: %v", err)
	}
}

// TestUnknownFieldAndVersionAreRefused covers the two failures a consumer must not
// absorb silently: a field the schema does not define, and a version this build
// cannot read.
func TestUnknownFieldAndVersionAreRefused(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(readTestFile(t, "valid.json"), &doc); err != nil {
		t.Fatal(err)
	}
	doc["learner_state"] = map[string]any{"concepts": 3}
	unknown, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, verr := Validate(unknown); verr == nil {
		t.Error("a release carrying learner state must be refused: the CLI owns content, not the learner")
	}

	doc = map[string]any{}
	if err := json.Unmarshal(readTestFile(t, "valid.json"), &doc); err != nil {
		t.Fatal(err)
	}
	doc["schema_version"] = "2.0"
	future, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, verr := Validate(future); verr == nil || !strings.Contains(verr.Error(), "2.0") {
		t.Errorf("a future version must be refused by name, got %v", verr)
	}
}

// TestSequenceNamesOnlyLiveClaims: the sequence is an order over claims the page
// carries, never a list a consumer has to resolve on its own.
func TestSequenceNamesOnlyLiveClaims(t *testing.T) {
	doc := strings.Replace(string(readTestFile(t, "valid.json")),
		`"sequence": ["c-az0hncye", "c-b77cy9md"]`,
		`"sequence": ["c-az0hncye", "c-ffffffff"]`, 1)
	if _, err := Validate([]byte(doc)); err == nil || !strings.Contains(err.Error(), "does not carry") {
		t.Errorf("a dangling sequence id must be refused, got %v", err)
	}
}

// TestSchemaDocumentMatchesTheMirror keeps the shipped JSON Schema and the Go
// types from drifting: every top-level key the schema defines exists in the
// mirror, and every required key is one the mirror writes.
func TestSchemaDocumentMatchesTheMirror(t *testing.T) {
	var schema struct {
		ID          string                     `json:"$id"`
		Required    []string                   `json:"required"`
		Properties  map[string]json.RawMessage `json:"properties"`
		Definitions map[string]struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(Schema, &schema); err != nil {
		t.Fatalf("schema.json does not parse: %v", err)
	}
	if !strings.HasSuffix(schema.ID, "/"+Version+".json") {
		t.Errorf("schema $id %q does not name version %s", schema.ID, Version)
	}

	// The mirror's keys come from the struct tags, which is the only list that
	// cannot drift from the type.
	want := jsonKeys(Release{})
	for key := range want {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("schema has no top-level property %q", key)
		}
	}
	for key := range schema.Properties {
		if !want[key] {
			t.Errorf("schema defines %q, which the mirror does not write", key)
		}
	}
	for _, req := range schema.Required {
		if !want[req] {
			t.Errorf("schema requires %q, unknown to the mirror", req)
		}
	}

	mirrored := jsonKeys(Page{})
	page, ok := schema.Definitions["page"]
	if !ok {
		t.Fatal("schema has no page definition")
	}
	for key := range page.Properties {
		if _, known := mirrored[key]; !known {
			t.Errorf("schema page property %q has no counterpart in the mirror", key)
		}
	}
	for _, req := range page.Required {
		if _, known := mirrored[req]; !known {
			t.Errorf("schema requires page.%s, which the mirror does not write", req)
		}
	}
}

// jsonKeys is the set of JSON names a struct writes, omitempty included: the tag,
// not the zero value, is what the schema must agree with.
func jsonKeys(v any) map[string]bool {
	keys := map[string]bool{}
	typ := reflect.TypeOf(v)
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			continue
		}
		keys[name] = true
	}
	return keys
}

func readTestFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
