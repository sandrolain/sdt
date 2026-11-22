//go:build !wasm

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/materialpackage"
	"github.com/sandrolain/sdt/internal/metacomp"
)

func TestPackageLinkedAssetsCopiesAndRewritesRelativeAsset(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context/assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context/assets/chart.svg"), []byte("<svg/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	assetMetadata := metacomp.New("context/assets/chart.svg", metacomp.Digest([]byte("<svg/>")))
	assetMetadata["privacy"] = materialpackage.PrivacyNone
	assetMetadataBytes, err := metacomp.Encode(assetMetadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context/assets/chart.svg.meta.yaml"), assetMetadataBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	var artifacts []materialpackage.InputArtifact
	data, err := packageLinkedAssets(root, "context/learning/lesson.md", "courses/lessons/lesson.md", []byte("![chart](../assets/chart.svg)"), map[string]string{}, map[string]bool{}, &artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 || artifacts[0].Type != "asset" || string(artifacts[0].Data) != "<svg/>" {
		t.Fatalf("unexpected asset artifacts: %#v", artifacts)
	}
	if strings.Contains(string(data), "../assets/chart.svg") || !strings.Contains(string(data), "../../assets/") {
		t.Fatalf("asset link was not rewritten relative to its exported document: %s", data)
	}
}

func TestPackageLinkedAssetsRejectsMissingAndPrivateAssets(t *testing.T) {
	root := t.TempDir()
	for _, link := range []string{"missing.png", "context/refs/source.png", "/tmp/private.png"} {
		t.Run(link, func(t *testing.T) {
			_, err := packageLinkedAssets(root, "context/learning/lesson.md", "courses/lessons/lesson.md", []byte("![image]("+link+")"), map[string]string{}, map[string]bool{}, &[]materialpackage.InputArtifact{})
			if err == nil {
				t.Fatalf("expected unsafe or missing asset %q to fail", link)
			}
		})
	}
	_, err := packageLinkedAssets(root, "context/learning/lesson.md", "courses/lessons/lesson.md", []byte("![](missing.png)"), map[string]string{}, map[string]bool{}, &[]materialpackage.InputArtifact{})
	if err == nil || !strings.Contains(err.Error(), "accessible description") {
		t.Fatalf("image without an accessible description should be refused, got %v", err)
	}
}

func TestPackageMarkdownRemovesSourceSpanAuthoringLine(t *testing.T) {
	data := []byte("---\nkind: lesson\ntitle: Example\nsummary: Portable\nproject: private\n---\n\nBody text.\n\n`source_span`: refs/converted/source.md@abc:1-2\n")
	got, err := packageMarkdown(data, "lesson", []string{"title", "summary"}, "", nil, "courses/lessons/example.md", map[string]packageMember{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "refs/") || strings.Contains(string(got), "source_span") || strings.Contains(string(got), "project:") {
		t.Fatalf("package retained private or authoring content: %s", got)
	}
}

func TestPackageMaterialDiagramsRenderWithPinnedPele(t *testing.T) {
	root := projectRoot(t)
	if _, err := os.Stat(filepath.Join(root, "web", "node_modules", "pele", "package.json")); err != nil {
		t.Skipf("pinned Pele is not installed: %v", err)
	}
	if _, err := findMaterialDiagramRuntime(); err != nil {
		t.Skip(err)
	}
	source := []byte("A simple flow:\n\n```mermaid\nflowchart LR\n A --> B\n```\n")
	got, artifacts, err := packageMaterialDiagrams(root, "course/topic.it", "materials/topic.it.md", source, map[string]string{"source": "v1"})
	if err != nil {
		t.Fatalf("render Mermaid: %v", err)
	}
	if strings.Contains(string(got), "```mermaid") || !strings.Contains(string(got), "![A simple flow](../assets/diagrams/course/topic.it/diagram-1.svg)") {
		t.Fatalf("rendered Markdown does not reference the accessible SVG: %s", got)
	}
	if len(artifacts) != 1 || artifacts[0].Type != "diagram-svg" || !strings.Contains(string(artifacts[0].Data), "<svg") {
		t.Fatalf("unexpected rendered artifacts: %#v", artifacts)
	}
	for i := range artifacts {
		artifacts[i].Privacy = materialpackage.PrivacyNone
	}
	items := append([]materialpackage.InputArtifact{{ID: "material:course/topic.it", Type: "guide", Privacy: materialpackage.PrivacyNone, Path: "materials/topic.it.md", Data: got}}, artifacts...)
	archive, err := materialpackage.Build(packageTargetPersonal, items, nil, "# Course\n", time.Now())
	if err != nil {
		t.Fatalf("build disconnected diagram package: %v", err)
	}
	manifest, err := materialpackage.Validate(archive)
	if err != nil {
		t.Fatalf("validate disconnected diagram package: %v", err)
	}
	if len(manifest.Artifacts) != 2 {
		t.Fatalf("manifest does not bind the guide and rendered SVG: %#v", manifest.Artifacts)
	}
}

func TestPackageMaterialDiagramsRejectsMissingCaptionAndMalformedSource(t *testing.T) {
	root := projectRoot(t)
	noCaption := []byte("```mermaid\nflowchart LR\n A --> B\n```\n")
	if _, _, err := packageMaterialDiagrams(root, "course/topic.it", "materials/topic.it.md", noCaption, nil); err == nil || !strings.Contains(err.Error(), "accessible caption") {
		t.Fatalf("diagram without caption should be rejected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "web", "node_modules", "pele", "package.json")); err != nil {
		t.Skipf("pinned Pele is not installed: %v", err)
	}
	if _, err := findMaterialDiagramRuntime(); err != nil {
		t.Skip(err)
	}
	malformed := []byte("A broken flow:\n\n```mermaid\nthis is not a diagram\n```\n")
	if _, _, err := packageMaterialDiagrams(root, "course/topic.it", "materials/topic.it.md", malformed, nil); err == nil || !strings.Contains(err.Error(), "failed to parse/render") {
		t.Fatalf("malformed Mermaid source should not be accepted as rendered: %v", err)
	}
}

func TestMaterialDiagramRuntimeReportsMissingRuntime(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := findMaterialDiagramRuntime(); err == nil || !strings.Contains(err.Error(), "rendering skipped") {
		t.Fatalf("missing runtime should be an explicit skipped result, got %v", err)
	}
}

func TestBuildSyllabusPackageRequiresClearedTarget(t *testing.T) {
	for target, want := range map[string]string{"": "--target is required", "public": "not cleared"} {
		t.Run(target, func(t *testing.T) {
			_, _, err := buildSyllabusPackage("syllabi/example", target, false, nil)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("target %q should be refused with %q, got %v", target, want, err)
			}
		})
	}
}

func TestPackageExcludesMaterialWithoutBoundBrief(t *testing.T) {
	writer := &syllabusPackageWriter{root: t.TempDir()}
	if writer.resolveMaterialBrief(materialFile{}, "course/topic.it") {
		t.Fatal("material without an exact brief binding must not enter the package")
	}
	if len(writer.exclusions) != 2 || writer.exclusions[0].ID != "material:course/topic.it" || writer.exclusions[1].ID != "brief:course/topic.it" {
		t.Fatalf("missing brief should report both the material and its missing binding: %#v", writer.exclusions)
	}
}

func TestPackageMaterialReadinessNamesBlockingDimensions(t *testing.T) {
	base := materialStatusRow{
		Freshness: renderFresh, Review: materialReviewRecorded,
		Classification: packageTargetPersonal, Deck: renderStale,
	}
	if reason := packageMaterialReadinessReason(base); reason != "" {
		t.Fatalf("guide-only export must ignore an unavailable deck: %s", reason)
	}
	base.Freshness = "stale"
	base.Review = "pending"
	base.Classification = "external"
	got := packageMaterialReadinessReason(base)
	for _, want := range []string{"freshness stale", "review pending", "classification external"} {
		if !strings.Contains(got, want) {
			t.Errorf("readiness reason %q is missing from %q", want, got)
		}
	}
}

func TestPackageDeckIsOptionalButStaleDeckIsReported(t *testing.T) {
	root := t.TempDir()
	writer := &syllabusPackageWriter{root: root}
	ref := materialFile{Path: filepath.Join(root, "guide.md"), Header: materialHeader{Lang: "it"}}
	if err := writer.addDeck(ref, "course/topic.it", materialStatusRow{Deck: renderFresh}); err != nil {
		t.Fatalf("an absent unplanned deck should not block a guide: %v", err)
	}
	if len(writer.exclusions) != 0 {
		t.Fatalf("absent deck should not add a blocking exclusion: %#v", writer.exclusions)
	}
	if err := os.WriteFile(strings.TrimSuffix(ref.Path, sdtMarkdownExt)+".slide"+sdtMarkdownExt, []byte("deck"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writer.addDeck(ref, "course/topic.it", materialStatusRow{Deck: renderStale}); err != nil {
		t.Fatal(err)
	}
	if len(writer.exclusions) != 1 || writer.exclusions[0].ID != "deck:course/topic.it" {
		t.Fatalf("stale deck should have a specific exclusion: %#v", writer.exclusions)
	}
}
