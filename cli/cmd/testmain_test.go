package cmd

import (
	"os"
	"testing"

	"github.com/sandrolain/sdt/internal/usage"
)

// TestMain disables the counted-usage log for the command tests by default.
// Every test that runs a command reaches the real exit path, so without this
// the suite would append records to this repository's own log and turn a real
// `sdt gain` report into a mix of work and test noise. A test that asserts the
// log's contents turns it back on explicitly.
func TestMain(m *testing.M) {
	if os.Getenv(usage.EnvOff) == "" {
		if err := os.Setenv(usage.EnvOff, "off"); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}
