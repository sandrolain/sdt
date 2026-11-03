package learn

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ── the freeze digest of one item's key ───────────────────────────────────────
//
// An item carries two digests and they answer two different questions. `hash`
// pins the wiki claim the item was written against: when the claim moves, the
// item is stale and must be re-read. `key_hash` pins the four cells that decide
// whether an answer is right — expected, rubric, grading and distractor — so a
// key edit re-freezes the item and a prompt edit does not.
//
// Two duties, two digests: freezing is an integrity rule for the quiz that is
// served, never a security boundary. A key in a Markdown file stays readable by
// anyone the file is readable to.

// keyDigestSeparator joins the four fields. It is the ASCII unit separator, a
// byte no cell can carry: a rubric may contain any punctuation, so any printable
// separator could be smuggled into a field and make two different keys collide.
const keyDigestSeparator = "\x1f"

// keyDigestLength is how much of the digest the corpus stores: 12 hex characters,
// the same width the claim digest uses, so the two columns read alike.
const keyDigestLength = 12

// KeyDigest is the freeze digest of one item's key: the sha256 of its expected
// answer, rubric, grading rule and distractor, joined by a separator no cell can
// contain. The fields are trimmed first, so trailing whitespace in a Markdown
// cell never moves the digest.
func KeyDigest(expected, rubric, grading, distractor string) string {
	joined := strings.Join([]string{
		strings.TrimSpace(expected),
		strings.TrimSpace(rubric),
		strings.TrimSpace(grading),
		strings.TrimSpace(distractor),
	}, keyDigestSeparator)
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])[:keyDigestLength]
}
