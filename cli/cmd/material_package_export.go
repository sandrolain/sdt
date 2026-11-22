//go:build !wasm

package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/materialbrief"
	"github.com/sandrolain/sdt/internal/materialpackage"
	"github.com/sandrolain/sdt/internal/materialprofile"
)

var materialPackageCmd = &cobra.Command{
	Use:   "package",
	Short: "Build a portable learning package",
}

var materialPackageExportCmd = &cobra.Command{
	Use:   "export <syllabus-id>",
	Short: "Export a syllabus and its linked learning materials as a ZIP",
	Long: `Export the syllabus, its themes and ordered lessons, related practice,
assessments, and ready guides/decks into one portable ZIP. The archive contains
relative Markdown links, a versioned JSON manifest, content digests and consumer
instructions. The manifest records privacy for every artifact; personal or real
data requires a cleared status and a short basis. Missing declarations block
the export. Current rights permit personal-use packages; another target needs
per-material rights review. Redirect stdout to a file, for example:
sdt material package export syllabi/software-architect --target personal > course.zip.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := getStringFlag(cmd, "target", false)
		archive, manifest, err := buildSyllabusPackage(args[0], target, getBoolFlag(cmd, "include-decks", false), loadLearnWiki(cmd))
		if err != nil {
			return err
		}
		outputBytes(cmd, archive)
		if len(manifest.Exclusions) > 0 {
			exitWithJunk(cmd, fmt.Sprintf("package built with %d exclusion(s)", len(manifest.Exclusions)))
		}
		return nil
	},
}

type packageMember struct {
	id    string
	title string
	path  string
	kind  string
}

var packageWikiLinkRE = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
var packageInlineIDRE = regexp.MustCompile("`([^`]+)`")
var packageSourceSpanLineRE = regexp.MustCompile(`(?m)^.*source_span[^\n]*(?:\n|$)`)
var packageMarkdownLinkRE = regexp.MustCompile(`(!?\[[^\]]*\]\()([^)]*)(\))`)
var packageAssetRE = regexp.MustCompile(`(?i)\.(png|jpe?g|gif|webp|svg|pdf|mp4|webm|mp3|wav|ogg)$`)

const (
	packageTargetPersonal    = "personal"
	packageSourceVersionKey  = "source"
	packageContentVersionKey = "content"
	packageFromLevelKey      = "from_level"
	packageToLevelKey        = "to_level"
)

func buildSyllabusPackage(syllabusID, target string, includeDecks bool, wiki *contextwiki.Builder) ([]byte, materialpackage.Manifest, error) {
	if err := validatePackageRequest(syllabusID, target, wiki); err != nil {
		return nil, materialpackage.Manifest{}, err
	}
	root, err := packageProjectRoot()
	if err != nil {
		return nil, materialpackage.Manifest{}, err
	}
	syllabus, themes, lessons, err := readSyllabusMembers(syllabusID, wiki)
	if err != nil {
		return nil, materialpackage.Manifest{}, err
	}
	writer := newSyllabusPackageWriter(root, wiki, includeDecks, syllabus, themes, lessons)
	if err := writer.addCourseDocuments(syllabus, themes); err != nil {
		return nil, materialpackage.Manifest{}, err
	}
	if err := writer.addLessonDocuments(lessons); err != nil {
		return nil, materialpackage.Manifest{}, err
	}
	if err := writer.addMaterials(syllabusID, lessons); err != nil {
		return nil, materialpackage.Manifest{}, err
	}
	return writer.build(target)
}

func validatePackageRequest(syllabusID, target string, wiki *contextwiki.Builder) error {
	if target == "" {
		return fmt.Errorf("--target is required")
	}
	if target != packageTargetPersonal {
		return fmt.Errorf("target %q is not cleared: current material rights allow personal-use packages only; external targets require per-material rights review", target)
	}
	if !strings.HasPrefix(syllabusID, "syllabi/") {
		return fmt.Errorf("syllabus id must start with syllabi/ (for example, syllabi/software-architect)")
	}
	if wiki == nil {
		return fmt.Errorf("load the wiki before exporting its syllabus")
	}
	return nil
}

func packageProjectRoot() (string, error) {
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	return filepath.Abs(root)
}

func readSyllabusMembers(syllabusID string, wiki *contextwiki.Builder) (learnCourseReport, []learnCourseReport, []string, error) {
	syllabus, err := learnCourseRead(wiki, syllabusID)
	if err != nil {
		return learnCourseReport{}, nil, nil, err
	}
	if syllabus.Kind != learnCourseSyllabus {
		return learnCourseReport{}, nil, nil, fmt.Errorf("%q is not a syllabus", syllabusID)
	}
	if len(syllabus.Refusals) > 0 || len(syllabus.Conflicts) > 0 {
		return learnCourseReport{}, nil, nil, fmt.Errorf("syllabus %s has membership refusals or order conflicts; resolve them before export", syllabusID)
	}
	if syllabus.Ready != syllabus.Total {
		return learnCourseReport{}, nil, nil, fmt.Errorf("syllabus %s has %d/%d teachable lessons; resolve readiness findings before export", syllabusID, syllabus.Ready, syllabus.Total)
	}
	themes := make([]learnCourseReport, 0, len(syllabus.Members))
	lessons := make([]string, 0, syllabus.Total)
	for _, member := range syllabus.Members {
		theme, readErr := learnCourseRead(wiki, member.ID)
		if readErr != nil {
			return learnCourseReport{}, nil, nil, readErr
		}
		if len(theme.Refusals) > 0 || len(theme.Conflicts) > 0 {
			return learnCourseReport{}, nil, nil, fmt.Errorf("theme %s has membership refusals or order conflicts; resolve them before export", theme.ID)
		}
		themes = append(themes, theme)
		for _, lesson := range theme.Members {
			lessons = append(lessons, lesson.ID)
		}
	}
	return syllabus, themes, lessons, nil
}

type syllabusPackageWriter struct {
	root               string
	wiki               *contextwiki.Builder
	includeDecks       bool
	members            map[string]packageMember
	artifacts          []materialpackage.InputArtifact
	exclusions         []materialpackage.Exclusion
	includedPractice   map[string]bool
	includedProfiles   map[string]bool
	assetDestinations  map[string]string
	includedAssetPaths map[string]bool
}

func newSyllabusPackageWriter(root string, wiki *contextwiki.Builder, includeDecks bool, syllabus learnCourseReport, themes []learnCourseReport, lessons []string) *syllabusPackageWriter {
	writer := &syllabusPackageWriter{
		root: root, wiki: wiki, includeDecks: includeDecks,
		members:          map[string]packageMember{},
		includedPractice: map[string]bool{}, includedProfiles: map[string]bool{},
		assetDestinations: map[string]string{}, includedAssetPaths: map[string]bool{},
	}
	writer.addMember(syllabus.ID, syllabus.Title, coursePackagePath("syllabi", syllabus.ID), "syllabus")
	for _, theme := range themes {
		writer.addMember(theme.ID, theme.Title, coursePackagePath("themes", theme.ID), "theme")
	}
	for _, lessonID := range lessons {
		title := lessonID
		if page := wiki.Resolve(lessonID); page != nil && page.Title != "" {
			title = page.Title
		} else if data, err := readPackageFile(root, learnLessonPath(lessonID)); err == nil {
			title = contextwiki.FrontmatterTitle(string(data))
		}
		writer.addMember(lessonID, title, coursePackagePath("lessons", lessonID), "lesson")
	}
	return writer
}

func (w *syllabusPackageWriter) addMember(id, title, dest, kind string) {
	w.members[id] = packageMember{id: id, title: title, path: dest, kind: kind}
}

func (w *syllabusPackageWriter) addDocument(id, kind, source, dest, section string, fields, sectionMembers []string) error {
	data, err := readPackageFile(w.root, source)
	if err != nil {
		return err
	}
	privacy, clearance, err := packagePrivacyFromDocument(data)
	if err != nil {
		return fmt.Errorf("classify %s %s: %w", kind, source, err)
	}
	sourceDigest := packageDigest(data)
	data, err = packageLinkedAssets(w.root, source, dest, data, w.assetDestinations, w.includedAssetPaths, &w.artifacts)
	if err != nil {
		return fmt.Errorf("package assets in %s: %w", source, err)
	}
	clean, err := packageMarkdown(data, kind, fields, section, sectionMembers, dest, w.members)
	if err != nil {
		return fmt.Errorf("package %s: %w", source, err)
	}
	w.artifacts = append(w.artifacts, materialpackage.InputArtifact{
		ID: id, Type: kind, Path: dest, Data: clean,
		SourceVersions: map[string]string{packageSourceVersionKey: sourceDigest}, Privacy: privacy, PrivacyClearance: clearance,
	})
	return nil
}

func (w *syllabusPackageWriter) addCourseDocuments(syllabus learnCourseReport, themes []learnCourseReport) error {
	if err := w.addDocument("syllabus:"+syllabus.ID, "syllabus", syllabus.Path,
		w.members[syllabus.ID].path, learnCourseThemesSection,
		[]string{ctxDocTitleKey, ctxMapNameSummary, "audience", packageFromLevelKey, packageToLevelKey, "outcome"}, memberIDs(syllabus.Members)); err != nil {
		return err
	}
	for _, theme := range themes {
		if err := w.addDocument("theme:"+theme.ID, "theme", theme.Path,
			w.members[theme.ID].path, learnCourseLessonsSection, []string{ctxDocTitleKey, ctxMapNameSummary}, memberIDs(theme.Members)); err != nil {
			return err
		}
	}
	return nil
}

func (w *syllabusPackageWriter) addLessonDocuments(lessons []string) error {
	for _, lessonID := range lessons {
		if err := w.addDocument("lesson:"+lessonID, "lesson", learnLessonPath(lessonID), w.members[lessonID].path, "",
			[]string{ctxDocTitleKey, ctxMapNameSummary, "objectives", "difficulty", "minutes"}, nil); err != nil {
			return err
		}
		if err := w.addAssessment(lessonID); err != nil {
			return err
		}
		if err := w.addPractice(lessonID); err != nil {
			return err
		}
	}
	return nil
}

func (w *syllabusPackageWriter) addAssessment(lessonID string) error {
	assessmentPath := learnAssessmentPath(lessonID)
	data, err := readPackageFile(w.root, assessmentPath)
	if err != nil {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "assessment:" + lessonID, Reason: "assessment bank is absent"})
		return nil
	}
	if stale, found := learnAssessmentStaleItem(lessonID); found {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "assessment:" + lessonID, Reason: fmt.Sprintf("assessment key for item %s is stale", stale.ID)})
		return nil
	}
	privacy, clearance, err := packagePrivacyFromDocument(data)
	if err != nil {
		return fmt.Errorf("classify assessment %s: %w", assessmentPath, err)
	}
	dest := path.Join("courses", "assessments", lessonID+".assessment.md")
	sourceDigest := packageDigest(data)
	data, err = packageLinkedAssets(w.root, assessmentPath, dest, data, w.assetDestinations, w.includedAssetPaths, &w.artifacts)
	if err != nil {
		return fmt.Errorf("package assets in assessment %s: %w", assessmentPath, err)
	}
	clean, err := packageMarkdown(data, "assessment", []string{ctxDocTitleKey, ctxMapNameSummary}, "", nil, dest, w.members)
	if err != nil {
		return fmt.Errorf("package assessment %s: %w", assessmentPath, err)
	}
	w.artifacts = append(w.artifacts, materialpackage.InputArtifact{
		ID: "assessment:" + lessonID, Type: "assessment", Path: dest, Data: clean,
		SourceVersions: map[string]string{packageSourceVersionKey: sourceDigest}, Privacy: privacy, PrivacyClearance: clearance,
	})
	return w.addQTI(lessonID, sourceDigest, privacy, clearance)
}

func (w *syllabusPackageWriter) addQTI(lessonID, assessmentDigest, privacy string, clearance *materialpackage.PrivacyClearance) error {
	items, ok := learnAssessmentItems(lessonID)
	if !ok || !hasChoiceInteraction(items) {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "qti:" + lessonID, Reason: "assessment has no supported closed-answer QTI interaction"})
		return nil
	}
	archive, err := qtiPackageForConcept(rootCmd, lessonID)
	if err != nil {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "qti:" + lessonID, Reason: err.Error()})
		return nil
	}
	w.artifacts = append(w.artifacts, materialpackage.InputArtifact{
		ID: "qti:" + lessonID, Type: "qti-package", Path: path.Join("qti", lessonID+".zip"), Data: archive,
		SourceVersions: map[string]string{"assessment": assessmentDigest}, Privacy: privacy, PrivacyClearance: clearance,
	})
	return nil
}

func (w *syllabusPackageWriter) addPractice(lessonID string) error {
	for _, practice := range learnPracticeOf(lessonID) {
		id := strings.TrimSuffix(filepath.Base(practice.Path), ".practice.md")
		if w.includedPractice[id] {
			continue
		}
		w.includedPractice[id] = true
		if err := w.addDocument("practice:"+id, "practice", practice.Path, path.Join("practice", id+".md"), "",
			[]string{ctxDocTitleKey, ctxMapNameSummary, "concepts"}, nil); err != nil {
			return err
		}
	}
	return nil
}

func (w *syllabusPackageWriter) addMaterials(syllabusID string, lessons []string) error {
	refs, err := packageMaterialsForLessons(w.root, syllabusID, lessons)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if err := w.addMaterial(ref); err != nil {
			return err
		}
	}
	return nil
}

func (w *syllabusPackageWriter) addMaterial(ref materialFile) error {
	materialID := materialIDFor(ref)
	row, reason := packageMaterialReadiness(ref)
	if reason != "" {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "material:" + materialID, Reason: reason})
		return nil
	}
	if !w.resolveMaterialBrief(ref, materialID) {
		return nil
	}
	privacy, clearance, err := packagePrivacyFromMaterial(ref.Metadata, materialArtifactGuide)
	if err != nil {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "material:" + materialID, Reason: err.Error()})
		return nil
	}
	guide, err := w.readCleanMaterial(ref, materialID)
	if err != nil {
		return err
	}
	guideDest := "materials/" + materialID + ".md"
	assetDestinations := clonePackageStringMap(w.assetDestinations)
	includedAssetPaths := clonePackageBoolMap(w.includedAssetPaths)
	var linkedAssets []materialpackage.InputArtifact
	guide, err = packageLinkedAssets(w.root, filepath.ToSlash(ref.Path), guideDest, guide, assetDestinations, includedAssetPaths, &linkedAssets)
	if err != nil {
		return fmt.Errorf("package assets in material %s: %w", materialID, err)
	}
	guide, diagramAssets, err := packageMaterialDiagrams(w.root, materialID, guideDest, guide, materialSourceVersions(ref))
	if err != nil {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "material:" + materialID, Reason: err.Error()})
		return nil
	}
	for i := range diagramAssets {
		diagramAssets[i].Privacy = privacy
		diagramAssets[i].PrivacyClearance = clearance
	}
	if findings := checkMaterialContents(ref.Path, string(guide), w.wiki); len(findings) > 0 {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "material:" + materialID, Reason: findings[0].Message})
		return nil
	}
	w.assetDestinations = assetDestinations
	w.includedAssetPaths = includedAssetPaths
	w.artifacts = append(w.artifacts, linkedAssets...)
	w.artifacts = append(w.artifacts, diagramAssets...)
	w.artifacts = append(w.artifacts, materialpackage.InputArtifact{ID: "material:" + materialID, Type: materialArtifactGuide, Path: guideDest, Data: guide, SourceVersions: materialSourceVersions(ref), Privacy: privacy, PrivacyClearance: clearance})
	if w.includeDecks {
		if err := w.addDeck(ref, materialID, row); err != nil {
			return err
		}
	}
	w.exclusions = append(w.exclusions,
		materialpackage.Exclusion{ID: "review:" + materialID, Reason: "author/evaluator review sidecar is private authoring data"},
		materialpackage.Exclusion{ID: "brief:" + materialID, Reason: "composition brief is authoring data and is not part of the reader package"},
	)
	return nil
}

func packageMaterialReadiness(ref materialFile) (materialStatusRow, string) {
	rows, err := materialStatusRows(ref.Header.Concept, ref.Header.Syllabus, ref.Header.Lang)
	if err != nil || len(rows) == 0 {
		if err == nil {
			err = fmt.Errorf("material readiness could not be resolved")
		}
		return materialStatusRow{}, err.Error()
	}
	row := rows[0]
	return row, packageMaterialReadinessReason(row)
}

func packageMaterialReadinessReason(row materialStatusRow) string {
	var reasons []string
	if row.Freshness != renderFresh {
		reasons = append(reasons, "freshness "+row.Freshness)
	}
	if row.Review != materialReviewRecorded {
		reasons = append(reasons, "review "+row.Review)
	}
	if row.Classification != packageTargetPersonal {
		reasons = append(reasons, "classification "+row.Classification)
	}
	return strings.Join(reasons, "; ")
}

func (w *syllabusPackageWriter) resolveMaterialBrief(ref materialFile, materialID string) bool {
	briefRef := metadataMap(ref.Metadata["material"])["brief"]
	if briefRef == nil {
		w.exclusions = append(w.exclusions,
			materialpackage.Exclusion{ID: "material:" + materialID, Reason: "material companion has no exact brief binding"},
			materialpackage.Exclusion{ID: "brief:" + materialID, Reason: "no bound composition brief is available"},
		)
		return false
	}
	binding := metadataMap(briefRef)
	briefID, revision := metadataString(binding["id"]), metadataString(binding["revision"])
	if briefID != materialID || revision == "" {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "material:" + materialID, Reason: "companion brief binding does not match the canonical material id"})
		return false
	}
	brief, err := materialbrief.Read(w.root, briefID, revision)
	if err != nil {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "material:" + materialID, Reason: "bound brief is unavailable: " + err.Error()})
		return false
	}
	if brief.Profile == nil {
		return true
	}
	profile, err := materialprofile.Read(w.root, brief.Profile.ID, brief.Profile.Revision)
	if err != nil {
		reason := "bound recipient profile is unavailable: " + err.Error()
		w.exclusions = append(w.exclusions,
			materialpackage.Exclusion{ID: "profile:" + brief.Profile.ID + "@" + brief.Profile.Revision, Reason: reason},
			materialpackage.Exclusion{ID: "material:" + materialID, Reason: reason},
		)
		return false
	}
	data, err := materialprofile.Encode(profile)
	if err != nil {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "material:" + materialID, Reason: "bound recipient profile cannot be encoded: " + err.Error()})
		return false
	}
	key := profile.ID + "@" + profile.Revision
	if !w.includedProfiles[key] {
		w.artifacts = append(w.artifacts, materialpackage.InputArtifact{
			ID: "profile:" + key, Type: "recipient-profile", Path: path.Join("recipient-profiles", profile.ID, profile.Revision+".yaml"), Data: data,
			SourceVersions: map[string]string{"profile": profile.Revision},
		})
		w.includedProfiles[key] = true
	}
	return true
}

func (w *syllabusPackageWriter) readCleanMaterial(ref materialFile, materialID string) ([]byte, error) {
	data, err := readPackageFile(w.root, filepath.ToSlash(ref.Path))
	if err != nil {
		return nil, err
	}
	cleanText, _ := cleanMaterialText(string(data), w.wiki)
	return []byte(cleanText), nil
}

func clonePackageStringMap(source map[string]string) map[string]string {
	copy := make(map[string]string, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}

func clonePackageBoolMap(source map[string]bool) map[string]bool {
	copy := make(map[string]bool, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}

func (w *syllabusPackageWriter) addDeck(ref materialFile, materialID string, row materialStatusRow) error {
	deckPath := strings.TrimSuffix(ref.Path, sdtMarkdownExt) + ".slide" + sdtMarkdownExt
	if _, err := os.Stat(deckPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if row.Deck != renderFresh {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "deck:" + materialID, Reason: "deck freshness " + row.Deck})
		return nil
	}
	data, err := readPackageFile(w.root, filepath.ToSlash(deckPath))
	if err != nil {
		return err
	}
	privacy, clearance, err := packagePrivacyFromMaterial(ref.Metadata, materialArtifactDeck)
	if err != nil {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "deck:" + materialID, Reason: err.Error()})
		return nil
	}
	clean, _, _ := cleanDeckText(string(data), w.wiki)
	dest := "materials/" + strings.TrimSuffix(materialID, "."+ref.Header.Lang) + "." + ref.Header.Lang + ".slide.md"
	assetDestinations := clonePackageStringMap(w.assetDestinations)
	includedAssetPaths := clonePackageBoolMap(w.includedAssetPaths)
	var linkedAssets []materialpackage.InputArtifact
	data, err = packageLinkedAssets(w.root, filepath.ToSlash(deckPath), dest, []byte(clean), assetDestinations, includedAssetPaths, &linkedAssets)
	if err != nil {
		return fmt.Errorf("package assets in deck %s: %w", materialID, err)
	}
	data, diagramAssets, err := packageMaterialDiagrams(w.root, materialID+".deck", dest, data, materialSourceVersions(ref))
	if err != nil {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "deck:" + materialID, Reason: err.Error()})
		return nil
	}
	for i := range diagramAssets {
		diagramAssets[i].Privacy = privacy
		diagramAssets[i].PrivacyClearance = clearance
	}
	if findings := checkMaterialContents(deckPath, string(data), w.wiki); len(findings) > 0 {
		w.exclusions = append(w.exclusions, materialpackage.Exclusion{ID: "deck:" + materialID, Reason: findings[0].Message})
		return nil
	}
	w.assetDestinations = assetDestinations
	w.includedAssetPaths = includedAssetPaths
	w.artifacts = append(w.artifacts, linkedAssets...)
	w.artifacts = append(w.artifacts, diagramAssets...)
	w.artifacts = append(w.artifacts, materialpackage.InputArtifact{ID: "deck:" + materialID, Type: "slide-deck", Path: dest, Data: data, SourceVersions: materialSourceVersions(ref), Privacy: privacy, PrivacyClearance: clearance})
	return nil
}

func (w *syllabusPackageWriter) build(target string) ([]byte, materialpackage.Manifest, error) {
	instructions := "# Course package\n\nOpen the syllabus under `courses/syllabi/` and follow its relative links. Lessons, practice notes, guides and available decks are Markdown files; Mermaid diagrams used by included materials are rendered to accessible SVG assets. Files under `qti/` are complete QTI 3.0 packages for an independent QTI reader. No SDT installation or agent runtime is required. The manifest records each included file's identity, source binding, SHA-256 digest and privacy declaration, and explains excluded items. Personal or real data appears only when its manifest clearance records a short review basis.\n"
	archive, err := materialpackage.Build(target, w.artifacts, w.exclusions, instructions, time.Now())
	if err != nil {
		return nil, materialpackage.Manifest{}, err
	}
	manifest, err := materialpackage.Validate(archive)
	if err != nil {
		return nil, materialpackage.Manifest{}, err
	}
	return archive, manifest, nil
}

func packageMaterialsForLessons(root, syllabusID string, lessons []string) ([]materialFile, error) {
	var refs []materialFile
	seen := map[string]bool{}
	for _, concept := range lessons {
		matches, err := matchMaterialFiles(concept, syllabusID, "")
		if err != nil {
			return nil, fmt.Errorf("material for %s: %w", concept, err)
		}
		for _, ref := range matches {
			if !seen[ref.Path] {
				refs = append(refs, ref)
				seen[ref.Path] = true
			}
		}
	}
	return refs, nil
}

func materialIDFor(ref materialFile) string {
	root := materialBaseDir()
	rel, err := filepath.Rel(root, ref.Path)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(filepath.ToSlash(rel), sdtMarkdownExt)
}

func materialSourceVersions(ref materialFile) map[string]string {
	versions := map[string]string{}
	material := metadataMap(ref.Metadata["material"])
	artifacts := metadataMap(material["artifacts"])
	guide := metadataMap(artifacts[materialArtifactGuide])
	claims := metadataSlice(guide[metadataClaimsKey])
	if len(claims) == 0 {
		claims = metadataSlice(material[metadataClaimsKey])
	}
	for _, raw := range claims {
		claim := metadataMap(raw)
		id, digest := metadataString(claim["id"]), metadataString(claim["digest"])
		if id != "" && digest != "" {
			versions[ref.Header.Concept+"#"+id] = digest
		}
	}
	return versions
}

func hasChoiceInteraction(items []learnLessonItem) bool {
	for _, item := range items {
		if item.Interaction != "" {
			return true
		}
	}
	return false
}

func coursePackagePath(prefix, id string) string {
	return path.Join("courses", prefix, strings.TrimPrefix(id, prefix+"/")) + ".md"
}

func memberIDs(members []learnCourseMember) []string {
	ids := make([]string, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.ID)
	}
	return ids
}

func readPackageFile(projectRoot, relative string) ([]byte, error) {
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(filepath.Clean(relative), ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("source path %q is not project-relative", relative)
	}
	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close() //nolint:errcheck // read-only handle cleanup
	data, err := root.ReadFile(filepath.Clean(filepath.FromSlash(relative)))
	if err != nil {
		return nil, fmt.Errorf("read source %s: %w", relative, err)
	}
	return data, nil
}

func packageDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// packageLinkedAssets copies each referenced local image/media/document into the
// archive and rewrites its link relative to the packaged Markdown file. Missing
// or unsafe local assets fail export so a required visual cannot silently vanish.
func packageLinkedAssets(root, source, dest string, data []byte, included map[string]string, includedPaths map[string]bool, artifacts *[]materialpackage.InputArtifact) ([]byte, error) {
	var firstErr error
	clean := packageMarkdownLinkRE.ReplaceAllFunc(data, func(raw []byte) []byte {
		if firstErr != nil {
			return raw
		}
		parts := packageMarkdownLinkRE.FindSubmatch(raw)
		link := strings.TrimSpace(string(parts[2]))
		rewritten, isAsset, err := packageAssetLink(root, source, dest, link, string(parts[1]), included, includedPaths, artifacts)
		if err != nil {
			firstErr = err
			return raw
		}
		if !isAsset {
			return raw
		}
		return []byte(string(parts[1]) + rewritten + string(parts[3]))
	})
	if firstErr != nil {
		return nil, firstErr
	}
	return clean, nil
}

func packageAssetLink(root, source, dest, link, prefix string, included map[string]string, includedPaths map[string]bool, artifacts *[]materialpackage.InputArtifact) (string, bool, error) {
	linkPath := strings.SplitN(strings.SplitN(link, "?", 2)[0], "#", 2)[0]
	if !packageAssetRE.MatchString(linkPath) || strings.HasPrefix(link, "//") || strings.Contains(link, "://") || strings.HasPrefix(link, "data:") {
		return "", false, nil
	}
	if strings.HasPrefix(prefix, "![") && strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(prefix, "!["), "](")) == "" {
		return "", false, fmt.Errorf("image asset %q has no accessible description", link)
	}
	if filepath.IsAbs(linkPath) {
		return "", false, fmt.Errorf("asset link %q is absolute", link)
	}
	assetSource := ""
	if strings.HasPrefix(filepath.ToSlash(linkPath), "context/") {
		assetSource = filepath.ToSlash(filepath.Clean(filepath.FromSlash(linkPath)))
	} else {
		assetSource = filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(filepath.FromSlash(source)), filepath.FromSlash(linkPath))))
	}
	if assetSource == ".." || strings.HasPrefix(assetSource, "../") {
		return "", false, fmt.Errorf("asset link %q escapes the project", link)
	}
	if strings.HasPrefix(assetSource, "context/refs/") || strings.HasPrefix(assetSource, "context/learning/") {
		return "", false, fmt.Errorf("asset link %q points into private or source material", link)
	}
	assetData, err := readPackageFile(root, assetSource)
	if err != nil {
		return "", false, fmt.Errorf("required asset %q: %w", link, err)
	}
	privacy, clearance, err := packagePrivacyFromAsset(root, assetSource)
	if err != nil {
		return "", false, fmt.Errorf("required asset %q: %w", link, err)
	}
	assetDest, exists := included[assetSource]
	if !exists {
		digest := packageDigest(assetData)
		name := filepath.Base(assetSource)
		assetDest = path.Join("assets", strings.TrimPrefix(digest, "sha256:")[:12], name)
		included[assetSource] = assetDest
		if !includedPaths[assetDest] {
			*artifacts = append(*artifacts, materialpackage.InputArtifact{
				ID: "asset:" + strings.TrimPrefix(digest, "sha256:") + ":" + name, Type: "asset", Path: assetDest, Data: assetData,
				SourceVersions: map[string]string{packageContentVersionKey: digest}, Privacy: privacy, PrivacyClearance: clearance,
			})
			includedPaths[assetDest] = true
		}
	}
	return packageRelativeLink(dest, assetDest), true, nil
}

func packageMarkdown(data []byte, kind string, fields []string, section string, sectionMembers []string, dest string, members map[string]packageMember) ([]byte, error) {
	fm, body := contextwiki.SplitFrontmatter(string(data))
	var metadata map[string]any
	if fm != "" {
		inner := strings.TrimSuffix(strings.TrimPrefix(fm, "---"), "---")
		if err := yaml.Unmarshal([]byte(inner), &metadata); err != nil {
			return nil, fmt.Errorf("parse %s frontmatter: %w", kind, err)
		}
	}
	filtered := map[string]any{}
	for _, field := range fields {
		if value, exists := metadata[field]; exists {
			filtered[field] = value
		}
	}
	frontmatter, err := yaml.Marshal(filtered)
	if err != nil {
		return nil, fmt.Errorf("encode portable %s metadata: %w", kind, err)
	}
	body = packageSourceSpanLineRE.ReplaceAllString(body, "")
	if section != "" {
		body = rewritePackageMembership(body, section, sectionMembers, dest, members)
	}
	body = packageWikiLinkRE.ReplaceAllStringFunc(body, func(raw string) string {
		match := packageWikiLinkRE.FindStringSubmatch(raw)
		id := strings.TrimPrefix(strings.TrimSpace(match[1]), "wiki/")
		member, ok := members[id]
		if !ok {
			if match[2] != "" {
				return strings.TrimSpace(match[2])
			}
			return id
		}
		label := match[2]
		if label == "" {
			label = member.title
		}
		return fmt.Sprintf("[%s](%s)", label, packageRelativeLink(dest, member.path))
	})
	body = packageInlineIDRE.ReplaceAllStringFunc(body, func(raw string) string {
		match := packageInlineIDRE.FindStringSubmatch(raw)
		id := strings.TrimPrefix(strings.TrimSpace(match[1]), "wiki/")
		if member, ok := members[id]; ok {
			return fmt.Sprintf("[%s](%s)", member.title, packageRelativeLink(dest, member.path))
		}
		return raw
	})
	clean := "---\n" + strings.TrimSpace(string(frontmatter)) + "\n---\n\n" + strings.TrimSpace(body) + "\n"
	for _, residue := range []string{"context/", "refs/", "<!--"} {
		if strings.Contains(clean, residue) {
			return nil, fmt.Errorf("export retains repository path or authoring marker %q", residue)
		}
	}
	return []byte(clean), nil
}

func rewritePackageMembership(body, section string, ids []string, from string, members map[string]packageMember) string {
	lines := strings.Split(body, "\n")
	active := false
	index := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			active = strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")) == section
			continue
		}
		if !active || index >= len(ids) || (!strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "* ")) {
			continue
		}
		member, ok := members[ids[index]]
		if !ok {
			continue
		}
		marker := "- "
		if strings.HasPrefix(trimmed, "* ") {
			marker = "* "
		}
		lines[i] = marker + fmt.Sprintf("[%s](%s)", member.title, packageRelativeLink(from, member.path))
		index++
	}
	return strings.Join(lines, "\n")
}

func packageRelativeLink(from, to string) string {
	rel, err := filepath.Rel(filepath.FromSlash(path.Dir(from)), filepath.FromSlash(to))
	if err != nil {
		return to
	}
	return filepath.ToSlash(rel)
}

func init() {
	materialPackageExportCmd.Flags().String("target", "", "package target; current rights allow personal")
	materialPackageExportCmd.Flags().Bool("include-decks", true, "include available reviewed slide decks")
	if err := materialPackageExportCmd.MarkFlagRequired("target"); err != nil {
		panic(err)
	}
	materialPackageCmd.AddCommand(materialPackageExportCmd)
	materialCmd.AddCommand(materialPackageCmd)
}
