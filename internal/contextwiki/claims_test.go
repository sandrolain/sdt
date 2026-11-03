package contextwiki

import (
	"strings"
	"testing"
)

func TestClaimsReadsNumberIDLineAndDigest(t *testing.T) {
	body := "## Claims\n\n1. One fact. (refs/spec.md@abc12345:3) {#claim-1} <!-- c-aaaaaaaa -->\n2. Two fact. {#claim-2} <!-- c-bbbbbbbb -->\n"
	claims := Claims(body)
	if len(claims) != 2 {
		t.Fatalf("expected 2 claims, got %d", len(claims))
	}
	first := claims[0]
	if first.Number != 1 || first.ID != "c-aaaaaaaa" || first.Line != 3 {
		t.Errorf("unexpected first claim: %+v", first)
	}
	if first.Text != "One fact." {
		t.Errorf("normalized text should drop the citation and both markers, got %q", first.Text)
	}
	if claims[1].ID != "c-bbbbbbbb" {
		t.Errorf("second id not read: %+v", claims[1])
	}
}

// TestClaimsDigestIgnoresPositionAndCitation is the property the digest exists
// for: the same statement at another number, with a re-pinned citation, keeps
// the same fingerprint; only the wording moves it.
func TestClaimsDigestIgnoresPositionAndCitation(t *testing.T) {
	base := "1. The pipeline routes by measured size. (refs/spec.md@abc12345:3) {#claim-1} <!-- c-aaaaaaaa -->"
	renumbered := "7. The pipeline routes by measured size. (refs/spec.md@abc12345:3) {#claim-7} <!-- c-aaaaaaaa -->"
	repinned := "1. The pipeline routes by measured size. (refs/spec.md@ffff9999:41) {#claim-1} <!-- c-aaaaaaaa -->"
	edited := "1. The pipeline routes by a measured size. (refs/spec.md@abc12345:3) {#claim-1} <!-- c-aaaaaaaa -->"

	first := Claims(base)[0].Digest
	if got := Claims(renumbered)[0].Digest; got != first {
		t.Errorf("renumbering changed the digest: %s vs %s", got, first)
	}
	if got := Claims(repinned)[0].Digest; got != first {
		t.Errorf("re-pinning the citation changed the digest: %s vs %s", got, first)
	}
	if got := Claims(edited)[0].Digest; got == first {
		t.Errorf("a wording change must move the digest")
	}
}

// TestClaimsBlockSpansWrappedLines covers the 16 corpus claims whose anchor sits
// on a citation-only line: the digest must cover the statement above it.
func TestClaimsBlockSpansWrappedLines(t *testing.T) {
	body := "## Claims\n\n1. A statement that wraps\n   onto a second line.\n   (refs/spec.md@abc12345:9)\n   {#claim-1} <!-- c-aaaaaaaa -->\n\n2. Next. {#claim-2} <!-- c-bbbbbbbb -->\n"
	claims := Claims(body)
	if len(claims) != 2 {
		t.Fatalf("expected 2 claims, got %d", len(claims))
	}
	if got, want := claims[0].Text, "A statement that wraps onto a second line."; got != want {
		t.Errorf("wrapped claim text = %q, want %q", got, want)
	}
	if got, want := claims[1].Text, "Next."; got != want {
		t.Errorf("second claim text = %q, want %q", got, want)
	}
}

func TestClaimsWithoutIDReportEmpty(t *testing.T) {
	claims := Claims("1. Fact. {#claim-1}\n")
	if len(claims) != 1 || claims[0].ID != "" {
		t.Fatalf("a claim without an id must report an empty id: %+v", claims)
	}
}

func TestClaimDigestIsStableAndShort(t *testing.T) {
	a := ClaimDigest("the same text")
	b := ClaimDigest("the same text")
	if a != b {
		t.Errorf("digest is not deterministic")
	}
	if len(a) != ClaimIDLength {
		t.Errorf("digest length = %d, want %d", len(a), ClaimIDLength)
	}
	if a == ClaimDigest("the same text ") {
		t.Errorf("callers must normalize before hashing; the digest itself must not trim")
	}
}

func TestMintClaimIDsStampsMissingAndKeepsExisting(t *testing.T) {
	body := "1. {#claim-1} one fact (refs/x.md@abc123:1)\n" +
		"2. {#claim-2} <!-- c-keepme12 --> two (refs/x.md@abc123:2)\n"
	got := MintClaimIDs(body, func() string { return "c-minted99" })
	if !strings.Contains(got, "{#claim-1} <!-- c-minted99 --> one fact") {
		t.Fatalf("missing id was not stamped:\n%s", got)
	}
	if !strings.Contains(got, "{#claim-2} <!-- c-keepme12 --> two") {
		t.Fatalf("existing id was disturbed:\n%s", got)
	}
	if again := MintClaimIDs(got, func() string { return "c-other000" }); again != got {
		t.Fatalf("minting is not idempotent:\n%s", again)
	}
}

func TestMintClaimIDsSkipsAnEmptyGenerator(t *testing.T) {
	body := "1. {#claim-1} a fact\n"
	if got := MintClaimIDs(body, func() string { return "" }); got != body {
		t.Fatalf("an empty id must be skipped, got:\n%s", got)
	}
	if got := MintClaimIDs(body, nil); got != body {
		t.Fatalf("a nil generator must leave the body alone, got:\n%s", got)
	}
}
