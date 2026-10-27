package cmd

import (
	"testing"

	"github.com/sandrolain/sdt/internal/acpfixture"
)

// TestAgentFixtureRefusesAnUnknownScript keeps the fixture's one contract: it plays a
// declared conversation, and a typo is an error rather than a silent default.
func TestAgentFixtureRefusesAnUnknownScript(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentFixtureCmd, nil, "--script", "nope"))
	})
}

// TestAgentFixtureIsHidden keeps `agent fixture` out of the help a project reads: it is
// verification infrastructure, not a verb of the workflow.
func TestAgentFixtureIsHidden(t *testing.T) {
	if !agentFixtureCmd.Hidden {
		t.Fatal("agent fixture must stay hidden")
	}
}

// TestAgentFixturePlaysEveryDeclaredScript runs the command for each script the package
// declares. The runner's stdin is at EOF, so Serve answers the handshake-less stream with
// its own end: the command accepted the script and returned.
func TestAgentFixturePlaysEveryDeclaredScript(t *testing.T) {
	for _, script := range []acpfixture.Script{
		acpfixture.ScriptAnswer,
		acpfixture.ScriptFail,
		acpfixture.ScriptAsk,
	} {
		t.Run(string(script), func(t *testing.T) {
			execute(t, agentFixtureCmd, nil, "--script", string(script))
		})
	}
}
