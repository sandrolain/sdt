package qti

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestWritePackageProducesDeterministicQTI3ChoiceBank(t *testing.T) {
	input := Package{ID: "bank-1", Title: "Bank", Items: []Item{{
		ID: "question-1", Title: `Question "one"`, Prompt: "Pick both", Objective: 2,
		Claim: "architecture/x#c-11111111", Provenance: "material", Interaction: "multiple-choice",
		Options: []Option{{ID: "a", Text: "A & one"}, {ID: "b", Text: "B"}, {ID: "c", Text: "C"}},
		Correct: []string{"a", "b"}, Scoring: "partial-credit",
	}}}
	first, err := WritePackage(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := WritePackage(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("identical assessment banks produced different package bytes")
	}
	files := unzip(t, first)
	manifest := string(files["imsmanifest.xml"])
	item := string(files["items/question-1.xml"])
	for _, want := range []string{"QTI Item Bank", "3.0.1", "claim:architecture/x#c-11111111", "objective:2", "provenance:material", `href="items/question-1.xml"`} {
		if !strings.Contains(manifest, want) {
			t.Errorf("manifest missing %q", want)
		}
	}
	for _, want := range []string{qtiNamespace, `title="Question &#34;one&#34;"`, `cardinality="multiple"`, `<qti-value>a</qti-value>`, `mapped-value="0.5"`, `mapped-value="-0.5"`, `A &amp; one`, `map_response.xml`} {
		if !strings.Contains(item, want) {
			t.Errorf("assessment item missing %q", want)
		}
	}
}

func TestWritePackageChoiceExplanationKeepsExplanationUnscored(t *testing.T) {
	got, err := WritePackage(Package{ID: "bank", Items: []Item{{
		ID: "question", Title: "Question", Prompt: "Choose", Interaction: "choice-explanation",
		Options: []Option{{ID: "yes", Text: "Yes"}, {ID: "no", Text: "No"}}, Correct: []string{"yes"},
		ExplanationPrompt: "Why?",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	item := string(unzip(t, got)["items/question.xml"])
	if !strings.Contains(item, `identifier="EXPLANATION" cardinality="single" base-type="string"`) || !strings.Contains(item, "Why?") {
		t.Fatal("separate explanation response was not emitted")
	}
	if strings.Contains(item, `qti-correct-response><qti-value>Why?</qti-value>`) {
		t.Fatal("explanation prompt was included in the automatic key")
	}
	if !strings.Contains(item, `<qti-match><qti-variable identifier="RESPONSE"/><qti-correct identifier="RESPONSE"/></qti-match>`) || strings.Contains(item, `<qti-correct identifier="EXPLANATION"/>`) {
		t.Fatal("response processing must grade the choice response only")
	}
}

func TestWritePackageSingleChoiceAndTrueFalse(t *testing.T) {
	input := Package{ID: "bank", Items: []Item{
		{ID: "single", Title: "Single", Prompt: "Pick one", Interaction: "single-choice", Options: []Option{{ID: "yes", Text: "Yes"}, {ID: "no", Text: "No"}}, Correct: []string{"yes"}},
		{ID: "boolean", Title: "Boolean", Prompt: "True?", Interaction: "true-false", Options: []Option{{ID: "true", Text: "True"}, {ID: "false", Text: "False"}}, Correct: []string{"true"}},
	}}
	archive, err := WritePackage(input)
	if err != nil {
		t.Fatal(err)
	}
	files := unzip(t, archive)
	if !strings.Contains(string(files["items/single.xml"]), `max-choices="1"`) || !strings.Contains(string(files["items/boolean.xml"]), `cardinality="single"`) {
		t.Fatal("single-choice or true-false interaction was not rendered as a single response")
	}
}

func TestWritePackageMultipleChoiceExactSet(t *testing.T) {
	archive, err := WritePackage(Package{ID: "bank", Items: []Item{{
		ID: "multiple", Title: "Multiple", Prompt: "Pick both", Interaction: "multiple-choice",
		Options: []Option{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}, {ID: "c", Text: "C"}}, Correct: []string{"a", "b"}, Scoring: "exact-set",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	item := string(unzip(t, archive)["items/multiple.xml"])
	if !strings.Contains(item, "match_correct.xml") || strings.Contains(item, "qti-mapping") {
		t.Fatal("exact-set multiple choice must use QTI's match-correct behavior")
	}
}

func TestWritePackageRejectsInvalidAnswerBindings(t *testing.T) {
	base := Item{ID: "question", Title: "Question", Prompt: "Choose", Interaction: "single-choice", Options: []Option{{ID: "a", Text: "A"}, {ID: "b", Text: "B"}}, Correct: []string{"a"}}
	for _, tc := range []struct {
		name string
		item Item
	}{
		{"unknown key", func() Item { i := base; i.Correct = []string{"missing"}; return i }()},
		{"duplicate option", func() Item { i := base; i.Options = append(i.Options, i.Options[0]); return i }()},
		{"partial credit single choice", func() Item { i := base; i.Scoring = "partial-credit"; return i }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := WritePackage(Package{ID: "bank", Items: []Item{tc.item}}); err == nil {
				t.Fatal("invalid item was accepted")
			}
		})
	}
}

func unzip(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte, len(r.File))
	for _, file := range r.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = content
	}
	return files
}
