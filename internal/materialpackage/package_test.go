package materialpackage

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestBuildAndValidatePortablePackage(t *testing.T) {
	archive, err := Build("personal", []InputArtifact{{
		ID:             "lesson:architecture/example",
		Type:           "lesson",
		Privacy:        PrivacyNone,
		Path:           "lessons/example.md",
		Data:           []byte("# Example lesson\n"),
		SourceVersions: map[string]string{"lesson": "sha256:abc"},
	}}, []Exclusion{{ID: "private-review", Reason: "author and evaluator notes are excluded"}}, "Open lessons/example.md.\n", time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Validate(archive)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Target != "personal" || len(manifest.Artifacts) != 1 || manifest.Artifacts[0].Path != "lessons/example.md" || manifest.Artifacts[0].Privacy != PrivacyNone {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestPackageRequiresExplicitPrivacyAndClearanceForSensitiveArtifacts(t *testing.T) {
	base := InputArtifact{ID: "asset:example", Type: "asset", Path: "assets/example.png", Data: []byte("image"), Privacy: PrivacyNone}
	if _, err := Build("personal", []InputArtifact{base}, nil, "read me", time.Now()); err != nil {
		t.Fatalf("privacy none should require no clearance: %v", err)
	}
	base.Privacy = PrivacyPersonal
	if _, err := Build("personal", []InputArtifact{base}, nil, "read me", time.Now()); err == nil || !strings.Contains(err.Error(), "privacy_clearance") {
		t.Fatalf("personal data without clearance should be rejected, got %v", err)
	}
	base.PrivacyClearance = &PrivacyClearance{Status: PrivacyCleared, Basis: "synthetic names and values removed"}
	archive, err := Build("personal", []InputArtifact{base}, nil, "read me", time.Now())
	if err != nil {
		t.Fatalf("cleared personal data should be accepted: %v", err)
	}
	manifest, err := Validate(archive)
	if err != nil {
		t.Fatal(err)
	}
	artifact := manifest.Artifacts[0]
	if artifact.Privacy != PrivacyPersonal || artifact.PrivacyClearance == nil || artifact.PrivacyClearance.Basis != "synthetic names and values removed" {
		t.Fatalf("manifest lost the privacy decision: %+v", artifact)
	}
}

func TestBuildRejectsUnsafeOrReservedPaths(t *testing.T) {
	for _, path := range []string{"../escape.md", "/absolute.md", "assets\\image.png", "manifest.json"} {
		t.Run(path, func(t *testing.T) {
			_, err := Build("personal", []InputArtifact{{ID: "a", Type: "guide", Privacy: PrivacyNone, Path: path, Data: []byte("x")}}, nil, "read me", time.Now())
			if err == nil {
				t.Fatalf("Build accepted path %q", path)
			}
		})
	}
}

func TestBuildRequiresMarkdownLinksToResolveInsidePackage(t *testing.T) {
	base := []InputArtifact{
		{ID: "guide", Type: "guide", Privacy: PrivacyNone, Path: "materials/guide.md", Data: []byte("[lesson](../courses/lesson.md)")},
		{ID: "lesson", Type: "lesson", Privacy: PrivacyNone, Path: "courses/lesson.md", Data: []byte("Lesson")},
	}
	if _, err := Build("personal", base, nil, "Read the course.", time.Unix(0, 0)); err != nil {
		t.Fatalf("resolvable relative link should be accepted: %v", err)
	}
	base[0].Data = []byte("[missing](../../private.md)")
	if _, err := Build("personal", base, nil, "Read the course.", time.Unix(0, 0)); err == nil {
		t.Fatal("link escaping the package should be rejected")
	}
}

func TestValidateRejectsUnlistedPrivateEntry(t *testing.T) {
	archive, err := Build("personal", []InputArtifact{{ID: "guide", Type: "guide", Privacy: PrivacyNone, Path: "guide.md", Data: []byte("guide")}}, nil, "read guide", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		var data bytes.Buffer
		if _, err := data.ReadFrom(input); err != nil {
			t.Fatal(err)
		}
		if err := input.Close(); err != nil {
			t.Fatal(err)
		}
		output, err := writer.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := output.Write(data.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
	output, err := writer.Create("review/private-notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write([]byte("private")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(buffer.Bytes()); err == nil || !strings.Contains(err.Error(), "unlisted entry") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsArtifactDigestMismatch(t *testing.T) {
	archive, err := Build("personal", []InputArtifact{{ID: "guide", Type: "guide", Privacy: PrivacyNone, Path: "guide.md", Data: []byte("guide")}}, nil, "read guide", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		var data bytes.Buffer
		if _, err := data.ReadFrom(input); err != nil {
			t.Fatal(err)
		}
		if err := input.Close(); err != nil {
			t.Fatal(err)
		}
		if file.Name == "guide.md" {
			data.Reset()
			data.WriteString("tampered")
		}
		output, err := writer.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := output.Write(data.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(buffer.Bytes()); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("Validate() error = %v", err)
	}
}
