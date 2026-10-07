package cmd

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/frontmatter"
)

// Durable, CLI-written state for the file-based multi-session handoff contract:
// a task file can be claimed (`claimed_by`/`claimed_at`) before work so a
// concurrent session reads the frontier and skips it. The claim is never
// hand-edited (HARD RULE 9); claim/release own the two keys.
const (
	ctxClaimedByKey = "claimed_by"
	ctxClaimedAtKey = "claimed_at"
)

var contextTaskClaimCmd = &cobra.Command{
	Use:   "claim",
	Short: "Claim a task file for this session",
	Long: "Record a durable claim (claimed_by/claimed_at) on a plan's task file and\n" +
		"mark it in progress, so a concurrent session reads the frontier from\n" +
		"`sdt context resume` and skips it. Release it with `task release`.\n\n" +
		"Examples:\n" +
		"  sdt context task claim --plan <plan-file> --agent opencode\n" +
		"  sdt context task release --plan <plan-file>",
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		phase, plan, err := taskTarget(cmd)
		exitWithError(cmd, err)
		path := taskFileForRef(phase, taskStreamFlag(cmd), plan)
		if _, err := os.Stat(path); err != nil {
			exitWithError(cmd, fmt.Errorf("no task file for %s (create it with `sdt context task add`)", plan))
			return
		}
		agent := getStringFlag(cmd, "agent", false)
		if agent == "" {
			agent = defaultClaimAgent()
		}
		if err := setTaskClaim(path, agent); err != nil {
			exitWithError(cmd, err)
			return
		}
		outputString(cmd, "claimed "+path+" by "+agent+"\n")
		cascadeAfterWrite(cmd, path)
	},
}

var contextTaskReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release a task file claim",
	Long: `Remove the claimed_by/claimed_at claim from a plan's task file and set it
back to pending, so it returns to the frontier.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		phase, plan, err := taskTarget(cmd)
		exitWithError(cmd, err)
		path := taskFileForRef(phase, taskStreamFlag(cmd), plan)
		if _, err := os.Stat(path); err != nil {
			exitWithError(cmd, fmt.Errorf("no task file for %s", plan))
			return
		}
		if err := clearTaskClaim(path); err != nil {
			exitWithError(cmd, err)
			return
		}
		outputString(cmd, "released "+path+"\n")
		cascadeAfterWrite(cmd, path)
	},
}

// defaultClaimAgent names the claiming session when --agent is omitted.
func defaultClaimAgent() string {
	if a := os.Getenv("SDT_AGENT"); a != "" {
		return a
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "session"
}

// setTaskClaim writes claimed_by/claimed_at and marks the file in progress.
func setTaskClaim(path, agent string) error {
	content, err := setTaskFrontmatter(path, map[string]string{
		ctxClaimedByKey: agent,
		ctxClaimedAtKey: contextNow().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	return writeContextDocFile(path, setTaskFileStatus(content, taskFileStatusInProgress))
}

// clearTaskClaim removes the claim and sets the file back to pending.
func clearTaskClaim(path string) error {
	content, err := unsetTaskFrontmatter(path, ctxClaimedByKey, ctxClaimedAtKey)
	if err != nil {
		return err
	}
	return writeContextDocFile(path, setTaskFileStatus(content, taskFileStatusPending))
}

// setTaskFrontmatter writes the given top-level keys with byte fidelity,
// re-parsing between writes because frontmatter.Block.Set is one-shot.
func setTaskFrontmatter(path string, kv map[string]string) (string, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- resolved task file
	if err != nil {
		return "", err
	}
	content := string(data)
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		block, err := frontmatter.Parse(content)
		if err != nil {
			return "", err
		}
		newYAML, err := block.Set(k, frontmatter.Scalar(kv[k]), ctxKeyOrder(""))
		if err != nil {
			return "", err
		}
		content = block.ReplaceBody(content, newYAML)
	}
	return content, nil
}

// unsetTaskFrontmatter removes the given top-level keys with byte fidelity.
func unsetTaskFrontmatter(path string, keys ...string) (string, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- resolved task file
	if err != nil {
		return "", err
	}
	content := string(data)
	for _, k := range keys {
		block, err := frontmatter.Parse(content)
		if err != nil {
			return "", err
		}
		newYAML, err := block.Unset(k)
		if err != nil {
			return "", err
		}
		content = block.ReplaceBody(content, newYAML)
	}
	return content, nil
}

func init() {
	contextTaskClaimCmd.Flags().String("agent", "", "Claiming agent/session id (default: $SDT_AGENT, $USER)")
}
