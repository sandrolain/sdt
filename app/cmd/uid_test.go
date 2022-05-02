package cmd

import (
	"regexp"
	"testing"
)

func TestUID(t *testing.T) {
	t.Run("UID UUID v4", func(t *testing.T) {
		out := execute(t, uidV4Cmd, []byte{})
		r := regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")
		if !r.MatchString(string(out)) {
			t.Fatalf("not a valid UUID v4 \"%v\"", string(out))
		}
	})

	t.Run("UID nano id", func(t *testing.T) {
		out := execute(t, uidNanoCmd, []byte{})
		r := regexp.MustCompile("^[A-Za-z0-9_-]{21}$")
		if !r.MatchString(string(out)) {
			t.Fatalf("not a valid nano-id \"%v\"", string(out))
		}
	})

	t.Run("KSUID", func(t *testing.T) {
		out := execute(t, uidKsCmd, []byte{})
		r := regexp.MustCompile("^[a-zA-Z0-9/+]{27}$")
		if !r.MatchString(string(out)) {
			t.Fatalf("not a valid KSUID \"%v\"", string(out))
		}
	})
}
