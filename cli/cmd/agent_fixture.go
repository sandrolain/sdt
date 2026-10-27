package cmd

import (
	"errors"
	"os"

	"github.com/sandrolain/sdt/internal/acpfixture"
	"github.com/spf13/cobra"
)

// agentFixtureCmd runs the scripted ACP agent over stdio. It is the offline cell an ACP
// client could not have: a client spawns this binary as its agent, so a conversation can
// be driven without a provider, a key or a paid turn.
//
// It is hidden: it is verification infrastructure, not a verb a reader works with, and the
// help stays about the commands a project uses.
var agentFixtureCmd = &cobra.Command{
	Use:    "fixture",
	Short:  "Run a scripted ACP agent over stdio",
	Hidden: true,
	Long: `Run a scripted ACP agent over stdio.

An ACP client spawns it like any agent — ` + "`agent.command: [\"sdt\", \"agent\", \"fixture\"]`" + ` in
.sdt.yaml — and gets the script instead of a provider: ` + "`answer`" + ` streams a long markdown
reply (code fence, table, callout, mermaid), ` + "`fail`" + ` refuses the turn the way a provider out
of quota does, ` + "`ask`" + ` sends an elicitation so a question card can be driven.

No network, no key, no socket. It exists so a scripted conversation runs offline.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		script := acpfixture.Script(getStringFlag(cmd, "script", false))
		switch script {
		case acpfixture.ScriptAnswer, acpfixture.ScriptFail, acpfixture.ScriptAsk:
		case "":
			script = acpfixture.ScriptAnswer
		default:
			exitWithError(cmd, errors.New(`--script must be "answer", "fail" or "ask"`))
			return
		}
		if repeat := getIntFlag(cmd, "repeat", false); repeat > 0 {
			acpfixture.Repeat = repeat
		}
		if err := acpfixture.Serve(os.Stdin, os.Stdout, script); err != nil {
			exitWithError(cmd, err)
			return
		}
	},
}

func init() {
	agentFixtureCmd.Flags().String("script", "answer", `Conversation to play: answer, fail or ask`)
	agentFixtureCmd.Flags().Int("repeat", 1, "How many times the answer repeats (overflows the thread)")
	agentCmd.AddCommand(agentFixtureCmd)
}
