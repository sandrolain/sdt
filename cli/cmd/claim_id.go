package cmd

import gonanoid "github.com/matoous/go-nanoid/v2"

// claimIDAlphabet is the set contextwiki.ClaimIDRegexp accepts: eight
// characters of [a-z0-9], the same shape the corpus migration used.
const claimIDAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// newClaimID is a variable so a test can pin the generated id. An error yields
// an empty id, which the minter skips rather than writing a broken anchor.
var newClaimID = func() string {
	id, err := gonanoid.Generate(claimIDAlphabet, 8)
	if err != nil {
		return ""
	}
	return "c-" + id
}
