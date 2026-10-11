// Agent refresh: re-render the generated surfaces without a full `agent init`.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var agentRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Re-render the generated instruction surfaces",
	Long: `Re-render the generated surfaces from the embedded templates.

  agent refresh    refresh context/instructions/, context/commands/,
                   .agents/skills/ and the AGENTS.md instructions block

It is the narrow alternative to ` + "`sdt agent init --force`" + `: it never asks
questions, never rewrites .sdt.yaml and never touches .gitignore, the write-once
` + "`<!-- sdt:begin:project -->`" + ` block, or the create-only seed files
(context/README.md, topics.yaml, categories.yaml, memo.yaml, todo.yaml,
briefing/onboarding.md).

Like ` + "`sdt agent init`" + `, it acts on the **current directory**: identity comes
from the .sdt.yaml there, and it does not walk up to a parent project. Run
` + "`sdt agent init`" + ` in a directory that has none.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadProjectConfig(sdtConfigFile)
		if cfg == nil {
			exitWithError(cmd, fmt.Errorf("no %s in this directory: run `sdt agent init` first", sdtConfigFile))
			return
		}
		results, err := agentRefreshGenerated(cfg, agentTargetPath(cmd), true, false)
		if err != nil {
			exitWithError(cmd, err)
		}
		outputFileResults(cmd, results)
	},
}

func init() {
	agentRefreshCmd.Flags().String("target", agentTargetDefault, "Output instruction file")
	agentCmd.AddCommand(agentRefreshCmd)
}
