package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// The custody report (`sdt context lint --custody`, option C of the
// refs-custody analysis): four corpus counts over the cited refs/ artifacts and
// the source sheets, reported as a summary rather than one finding per file, so
// the lint baseline cannot absorb the class. It is the check a deletion is
// preceded by — what the file's removal would break, and which custody
// transitions are half-done — made executable.
//
// The counts:
//
//  1. cited artifacts with no custody sheet — a citation whose bytes have no
//     recorded capture_path or sha256 pin anywhere under context/sources/;
//  2. gone sheets whose capture_path still exists — the transition says the
//     file was removed, the file disagrees;
//  3. sheets whose sha256 disagrees with the file they pin — an immutability
//     violation: the pinned bytes changed under a sheet that still claims them;
//  4. gone sheets with no restore command — a recorded loss with no recovery
//     path.
//
// The report states what it cost (documents scanned, sheets read, files hashed,
// duration): the walk is the price of the answer, on demand only.

// ctxSourceSheetCapturePath and ctxSourceSheetRestore are the sheet keys the
// custody report reads; the rest of a sheet is prose the report never guesses
// at (context_lint_source.go owns the three keys with a checkable form).
const (
	ctxSourceSheetCapturePath = "capture_path"
	ctxSourceSheetRestore     = "restore"
	ctxSourceSheetSHA256      = "sha256"
)

// ctxCustodyReport is the four-count summary plus the run's own cost.
type ctxCustodyReport struct {
	CitedNoSheet       int      `json:"cited_without_sheet" yaml:"cited_without_sheet"`
	Uncovered          []string `json:"cited_artifacts_without_sheet,omitempty" yaml:"cited_artifacts_without_sheet,omitempty"`
	GoneWithLiveFile   int      `json:"gone_with_capture_path_present" yaml:"gone_with_capture_path_present"`
	DigestDisagreement int      `json:"sha256_disagreement" yaml:"sha256_disagreement"`
	GoneNoRestore      int      `json:"gone_without_restore" yaml:"gone_without_restore"`
	// Cost: what the one pass over the corpus and refs/ paid.
	DocsScanned int    `json:"documents_scanned" yaml:"documents_scanned"`
	SheetsRead  int    `json:"sheets_read" yaml:"sheets_read"`
	FilesHashed int    `json:"files_hashed" yaml:"files_hashed"`
	Duration    string `json:"duration" yaml:"duration"`
}

// String renders the text summary, one count per line so a scan reads it in
// order and the cost line closes it.
func (r ctxCustodyReport) String() string {
	return fmt.Sprintf(`[custody] cited artifacts without a sheet: %d
[custody] gone sheets whose capture_path still exists: %d
[custody] sheets whose sha256 disagrees with the file they pin: %d
[custody] gone sheets without a restore command: %d
[custody] cost: %d documents scanned, %d sheets read, %d files hashed in %s`,
		r.CitedNoSheet, r.GoneWithLiveFile, r.DigestDisagreement, r.GoneNoRestore,
		r.DocsScanned, r.SheetsRead, r.FilesHashed, r.Duration)
}

// ctxSourceSheet is the frontmatter slice of one source sheet the custody
// report needs: the status, the pinned file, the pin and the recovery path.
// capture stays as the sheet wrote it — a context-relative path, the same form
// a citation uses (`refs/…`) — so file resolution and citation matching share
// one address.
type ctxSourceSheet struct {
	status  string
	capture string // context-relative, as written (typically `refs/…`)
	extract string
	sha     string // lowercase 64-hex, or "" when absent/malformed
	restore string
}

// lintCustodyReport walks the corpus once: every indexed document through the
// citation rules (the same strip + regexp the citation lint applies), every
// sheet under context/sources/, and each pinned file hashed once.
func lintCustodyReport() ctxCustodyReport {
	start := time.Now()
	report := ctxCustodyReport{}

	// (1) The cited artifacts: distinct refs/<path> targets with every digest
	// prefix cited against them.
	cited := lintCitedArtifacts(&report)

	// (2–4) The sheets.
	sheets := lintSourceSheets(&report)

	sheetCovers := func(ref string, prefixes []string) bool {
		for _, sh := range sheets {
			if ctxCustodyRelPath(sh.capture) == ref {
				return true
			}
			if ctxCustodyRelPath(sh.extract) == ref {
				return true
			}
			if sh.sha != "" {
				for _, prefix := range prefixes {
					if strings.HasPrefix(sh.sha, prefix) {
						return true
					}
				}
			}
		}
		return false
	}
	for ref, prefixes := range cited {
		if !sheetCovers(ref, prefixes) {
			report.CitedNoSheet++
			report.Uncovered = append(report.Uncovered, "refs/"+ref)
		}
	}
	slices.Sort(report.Uncovered)

	report.Duration = time.Since(start).Round(time.Millisecond).String()
	return report
}

// lintCitedArtifacts scans every corpus document and returns the distinct cited
// refs/ artifacts (normalized to context-relative, refs/-free paths) mapped to
// the digest prefixes cited against each. It counts the documents it read in
// report.DocsScanned. A citation that is not a claim — inside a code span, a
// fenced block, or the unresolved `@sha256:` form — is never counted, through
// the same rules the citation lint applies.
func lintCitedArtifacts(report *ctxCustodyReport) map[string][]string {
	cited := map[string][]string{}
	for _, dir := range ctxIndexDirs {
		files, err := dirFilesFor(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			data, err := os.ReadFile(f) //#nosec G304 -- fixed corpus path
			if err != nil {
				continue
			}
			report.DocsScanned++
			body := stripInlineCode(stripFencedCode(string(frontmatterBody(data))))
			for _, m := range ctxCitationRegexp.FindAllStringSubmatch(body, -1) {
				if m[2] != "" { // the `@sha256:` form carries no resolvable path
					continue
				}
				ref := ctxCustodyRelPath(m[1])
				if !slices.Contains(cited[ref], m[3]) {
					cited[ref] = append(cited[ref], m[3])
				}
			}
		}
	}
	return cited
}

// lintSourceSheets reads every source sheet under context/sources/ and returns
// its custody-relevant fields. It counts the sheets it read in
// report.SheetsRead, and runs the three per-sheet counts (2, 3, 4) in place.
func lintSourceSheets(report *ctxCustodyReport) []ctxSourceSheet {
	files, err := dirFiles(sdtSourcesDir)
	if err != nil {
		return nil
	}
	var sheets []ctxSourceSheet
	for _, f := range files {
		data, err := os.ReadFile(f) //#nosec G304 -- fixed corpus path
		if err != nil {
			continue
		}
		content := string(data)
		if kind := parseFrontmatterField(content, ctxFrontmatterKind); kind != ctxTypeSource {
			continue
		}
		report.SheetsRead++
		sheet := ctxSourceSheet{
			status:  strings.TrimSpace(parseFrontmatterField(content, ctxMapStatus)),
			capture: strings.TrimSpace(filepath.ToSlash(parseFrontmatterField(content, ctxSourceSheetCapturePath))),
			extract: strings.TrimSpace(filepath.ToSlash(parseFrontmatterField(content, "extract"))),
			sha:     strings.TrimSpace(parseFrontmatterField(content, ctxSourceSheetSHA256)),
			restore: strings.TrimSpace(parseFrontmatterField(content, ctxSourceSheetRestore)),
		}
		if sheet.sha != "" && !ctxSourceSHA256Regexp.MatchString(sheet.sha) {
			sheet.sha = "" // a malformed pin pins nothing
		}
		sheets = append(sheets, sheet)

		gone := sheet.status == ctxSourceStatusGone
		// (2) A gone transition whose file is still on disk: the sheet and the
		// tree disagree about whether the capture exists.
		if gone && sheet.capture != "" && sheetFileExists(sheet.capture) {
			report.GoneWithLiveFile++
		}
		// (3) The pin disagrees with the file it pins: hashed bytes are the
		// immutability contract of a sheet.
		if sheet.capture != "" && sheet.sha != "" {
			if actual, ok := ctxRefSHA256(sheet.capture); ok {
				report.FilesHashed++
				if actual != sheet.sha {
					report.DigestDisagreement++
				}
			}
		}
		// (4) A recorded loss with no recovery path.
		if gone && sheet.restore == "" {
			report.GoneNoRestore++
		}
	}
	return sheets
}

// ctxCustodyRelPath normalizes a cited ref or a sheet capture_path to a
// context-relative slash path without the refs/ prefix: `refs/a/b.md` (the
// citation form) and `a/b.md` (a sheet's shorthand) name the same artifact.
func ctxCustodyRelPath(p string) string {
	p = filepath.ToSlash(strings.TrimSpace(p))
	return strings.TrimPrefix(p, "refs/")
}

// sheetFileExists reports whether a context-relative capture path resolves to
// an existing file.
func sheetFileExists(rel string) bool {
	fi, err := os.Stat(filepath.Join(sdtWorkDir, filepath.FromSlash(rel)))
	return err == nil && !fi.IsDir()
}

// ctxRefSHA256 reads a context-relative file and returns its full SHA-256 hex
// digest, or ok=false when the file is absent or unreadable.
func ctxRefSHA256(rel string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(sdtWorkDir, filepath.FromSlash(rel))) //#nosec G304 -- path from a source sheet inside context/
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), true
}
