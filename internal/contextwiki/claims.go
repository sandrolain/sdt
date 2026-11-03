// Claim identity and digest: the stable id a claim carries beside its readable
// anchor, and the fingerprint of the text it was written with.
//
// The number in `{#claim-7}` is a position: inserting or deleting a claim moves
// it, so an artifact that cites it (a lesson item, a portal's evidence) would
// silently follow a different fact. The id written as `<!-- c-3f9a2b1d -->` on
// the same claim never moves. The digest exists for the other half — the text
// itself changing under an unchanged id — and is computed over the claim's
// normalized block, never over the citation, so re-pinning a source's sha does
// not read as a content change.
//
// The parsing lives here, in the shared builder, so the CLI lint, the lesson
// validator and any exporter read one implementation (ADR-0003).
package contextwiki

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
)

// ClaimIDLength is the number of hex characters a claim digest keeps. Twelve is
// short enough to paste into an item cell and long enough that a collision is
// not a practical concern inside one page.
const ClaimIDLength = 12

// ClaimIDRegexp matches the stable id comment a claim carries beside its anchor.
// The form is deliberately the checklist's HTML comment (`<!-- c43 -->`), with a
// dash so the two id spaces never collide, and it renders as nothing.
var ClaimIDRegexp = regexp.MustCompile(`<!--\s*(c-[a-z0-9]{8})\s*-->`)

// claimCitationRegexp matches a hash-anchored citation, with or without its
// surrounding parentheses, so removing it never leaves an empty pair behind.
var claimCitationRegexp = regexp.MustCompile(`\(\s*refs/[A-Za-z0-9_./-]+@[0-9a-f]{6,}(?::[0-9]+(?:-[0-9]+)?)?\s*\)|refs/[A-Za-z0-9_./-]+@[0-9a-f]{6,}(?::[0-9]+(?:-[0-9]+)?)?`)

// claimListMarkerRegexp matches a list marker at the start of a line. The marker
// is position, not fact: keeping it would turn a reordered claim into a digest
// change.
var claimListMarkerRegexp = regexp.MustCompile(`(?m)^[ \t]*(?:[0-9]+[.)]|[-*+])[ \t]+`)

// Claim is one claim of a page: its readable number, its stable id, the line its
// anchor sits on, the normalized text the digest is taken over, and the digest.
type Claim struct {
	Number int    `json:"number" yaml:"number"`
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
	Line   int    `json:"line" yaml:"line"`
	Text   string `json:"text" yaml:"text"`
	Digest string `json:"digest" yaml:"digest"`
}

// Claims reads a page body's claims in document order. A claim's block runs from
// the line after the previous claim's block (or the nearest blank line above) to
// the anchor's own line, so a claim written over several lines — including the
// ones that keep their anchor on a citation-only line — yields the whole text.
func Claims(body string) []Claim {
	lines := strings.Split(body, "\n")
	anchors := ClaimRegexp.FindAllStringIndex(body, -1)
	if len(anchors) == 0 {
		return nil
	}
	claims := make([]Claim, 0, len(anchors))
	previousEnd := -1
	for _, loc := range anchors {
		line := strings.Count(body[:loc[0]], "\n")
		start := claimBlockStart(lines, line, previousEnd)
		block := strings.Join(lines[start:line+1], "\n")
		number, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(body[loc[0]:loc[1]], "{#claim-"), "}"))
		if err != nil {
			number = 0
		}
		text := NormalizeClaimBlock(block)
		claims = append(claims, Claim{
			Number: number,
			ID:     claimIDOf(block),
			Line:   line + 1,
			Text:   text,
			Digest: ClaimDigest(text),
		})
		previousEnd = line
	}
	return claims
}

// Claims returns the page's claims.
func (p *Page) Claims() []Claim { return Claims(p.Body) }

// claimBlockStart walks up from the anchor line while the line above is part of
// the same block: non-empty, and not the end of the previous claim.
func claimBlockStart(lines []string, anchorLine, previousEnd int) int {
	start := anchorLine
	for start-1 > previousEnd && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	return start
}

// claimIDOf returns the stable id carried inside one claim block, or "".
func claimIDOf(block string) string {
	if m := ClaimIDRegexp.FindStringSubmatch(block); m != nil {
		return m[1]
	}
	return ""
}

// NormalizeClaimBlock turns a claim block into the digest's input: the anchor,
// the id comment, the citations and the list marker removed, whitespace
// collapsed. What remains is the statement, which is what must not change.
func NormalizeClaimBlock(block string) string {
	s := ClaimIDRegexp.ReplaceAllString(block, " ")
	s = ClaimRegexp.ReplaceAllString(s, " ")
	s = claimCitationRegexp.ReplaceAllString(s, " ")
	s = claimListMarkerRegexp.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// MintClaimIDs stamps a stable id after every claim anchor that lacks one,
// leaving an existing id untouched. gen supplies the id, so the caller owns the
// alphabet and a test can pin it; an empty id is skipped rather than written.
//
// The rule mirrors the checklist anchors `sdt context check` stamps on write: a
// written claim never arrives without an identity, and an id already present is
// never regenerated — a rewritten claim keeps its id and its digest changes,
// which is what `learn lesson --validate` reports as stale.
func MintClaimIDs(body string, gen func() string) string {
	if gen == nil {
		return body
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		loc := ClaimRegexp.FindStringIndex(line)
		if loc == nil {
			continue
		}
		if ClaimIDRegexp.MatchString(line[loc[1]:]) {
			continue
		}
		id := gen()
		if id == "" {
			continue
		}
		lines[i] = line[:loc[1]] + " <!-- " + id + " -->" + line[loc[1]:]
	}
	return strings.Join(lines, "\n")
}

// ClaimDigest is the first ClaimIDLength hex characters of the sha256 of a
// normalized claim block.
func ClaimDigest(normalized string) string {
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])[:ClaimIDLength]
}
