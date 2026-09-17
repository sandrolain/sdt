package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func resolveGitIgnoreMode(cmd *cobra.Command, yes bool) string {
	raw := strings.ToLower(strings.TrimSpace(getStringFlag(cmd, "gitignore", false)))
	if raw != "" {
		switch raw {
		case gitIgnoreModeNone, gitIgnoreModeTmp, gitIgnoreModeDocs, gitIgnoreModeWork, gitIgnoreModeContext:
			return raw
		}
		exitWithError(cmd, fmt.Errorf("invalid --gitignore %q (want one of none|tmp|docs|work|context)", raw))
	}
	if yes || !stdinIsTTY() {
		return gitIgnoreModeWork
	}
	if !agentPromptBool(cmd, false, "Add context working directories to .gitignore?", true) {
		return gitIgnoreModeNone
	}
	return agentPromptGitIgnoreMode(cmd, false, gitIgnoreModeWork)
}

// agentPromptBool asks a yes/no question on the terminal. When yes is set, or
// stdin is not a terminal, the default is returned without prompting.

func agentPromptGitIgnoreMode(cmd *cobra.Command, yes bool, def string) string {
	if yes || !stdinIsTTY() {
		return def
	}
	if _, ferr := fmt.Fprintln(cmd.ErrOrStderr(), "Which context entries should be added to .gitignore?"); ferr != nil {
		_ = ferr
	}
	if _, ferr := fmt.Fprintf(cmd.ErrOrStderr(), "  1) %s\n", gitIgnoreTmpEntry); ferr != nil {
		_ = ferr
	}
	if _, ferr := fmt.Fprintf(cmd.ErrOrStderr(), "  2) %s\n", gitIgnoreDocsEntry); ferr != nil {
		_ = ferr
	}
	if _, ferr := fmt.Fprintf(cmd.ErrOrStderr(), "  3) %s (entire working directory)\n", gitIgnoreContextEntry); ferr != nil {
		_ = ferr
	}
	if _, ferr := fmt.Fprint(cmd.ErrOrStderr(), "Comma-separated (1,2,3) or keywords (tmp, docs, context) [1,2]: "); ferr != nil {
		_ = ferr
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && err != io.EOF {
		return def
	}
	sel := strings.ToLower(strings.TrimSpace(line))
	if sel == "" {
		return def
	}
	selected := map[string]bool{}
	for _, part := range strings.Split(sel, ",") {
		choice := strings.TrimSpace(part)
		switch choice {
		case "1", gitIgnoreModeTmp:
			selected[gitIgnoreModeTmp] = true
		case "2", gitIgnoreModeDocs:
			selected[gitIgnoreModeDocs] = true
		case "3", gitIgnoreModeContext, "all":
			return gitIgnoreModeContext
		case gitIgnoreModeWork:
			return gitIgnoreModeWork
		}
	}
	if selected[gitIgnoreModeTmp] && selected[gitIgnoreModeDocs] {
		return gitIgnoreModeWork
	}
	if selected[gitIgnoreModeTmp] {
		return gitIgnoreModeTmp
	}
	if selected[gitIgnoreModeDocs] {
		return gitIgnoreModeDocs
	}
	return def
}

const gitIgnoreTmpEntry = "context/tmp/"

// gitIgnoreDocsEntry keeps generated agent docs out of version control. The
// reference is regenerated per binary version, so it is never committed.

const gitIgnoreDocsEntry = "context/sdtdocs/"

// gitIgnoreContextEntry ignores the entire context/ working directory,
// including plans, work logs, notes and instruction files.

const gitIgnoreContextEntry = "context/"

// gitIgnoreWorkEntries lists the context/ entries ensured by default (work).

var gitIgnoreWorkEntries = []string{gitIgnoreTmpEntry, gitIgnoreDocsEntry}

// gitIgnore block markers bound the sdt-managed .gitignore entries so later
// runs can update them in place and the block is distinguishable from
// hand-written entries.
const (
	gitIgnoreBlockStart = "# sdt:start"
	gitIgnoreBlockEnd   = "# sdt:end"
)

// gitIgnoreEntriesForMode maps a gitignore mode to the entries it ensures.
// An empty result means no .gitignore changes (none).

func gitIgnoreEntriesForMode(mode string) []string {
	switch mode {
	case gitIgnoreModeNone:
		return nil
	case gitIgnoreModeTmp:
		return []string{gitIgnoreTmpEntry}
	case gitIgnoreModeDocs:
		return []string{gitIgnoreDocsEntry}
	case gitIgnoreModeContext:
		return []string{gitIgnoreContextEntry}
	default:
		return gitIgnoreWorkEntries
	}
}

// ensureGitIgnore ensures the .gitignore file located in the current working
// directory (the .sdt.yaml location) ignores the context working
// directories selected by mode (tmp, docs, work, context). The file is
// maintained even outside a git repository and parent directories are never
// resolved. Entries live inside a `# sdt:start` / `# sdt:end` block: the block
// is appended when absent, missing entries are inserted inside an existing
// block, and nothing changes when every entry is already present. It returns
// nil only when mode is none.

func ensureGitIgnore(mode string) *FileResult {
	entries := gitIgnoreEntriesForMode(mode)
	if len(entries) == 0 {
		return nil
	}
	path := filepath.Join(".", ".gitignore")
	res := &FileResult{Path: path}
	existing := ""
	if data, err := os.ReadFile(path); err == nil { //#nosec G304 -- user project directory derived from cwd
		existing = string(data)
	} else if !os.IsNotExist(err) {
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	missing := gitIgnoreMissingEntries(existing, entries)
	if len(missing) == 0 {
		res.Status = statusSkipped
		res.Reason = "entries already present"
		return res
	}
	inner := strings.Join(missing, "\n")
	content := existing
	if strings.Contains(content, gitIgnoreBlockEnd) {
		content = strings.Replace(content, gitIgnoreBlockEnd, inner+"\n"+gitIgnoreBlockEnd, 1)
	} else {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += gitIgnoreBlockStart + "\n" + inner + "\n" + gitIgnoreBlockEnd + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //#nosec G306,G703 -- user project directory derived from cwd
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	if existing == "" {
		res.Status = statusCreated
	} else {
		res.Status = statusUpdated
	}
	return res
}

// gitIgnoreMissingEntries returns the entries not already present as a line in
// content (tolerating a missing trailing slash), independent of the sdt block.

func gitIgnoreMissingEntries(content string, entries []string) []string {
	var missing []string
	for _, entry := range entries {
		if !gitIgnoreHasEntry(content, entry) {
			missing = append(missing, entry)
		}
	}
	return missing
}

// gitIgnoreHasEntry reports whether content already contains entry as a line,
// tolerating a missing trailing slash and surrounding whitespace.

func gitIgnoreHasEntry(content, entry string) bool {
	base := strings.TrimSuffix(entry, "/")
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == entry || line == base {
			return true
		}
	}
	return false
}

// agentMergeTarget ensures AGENTS.md carries the instructions block (refreshed
// with --force) plus the write-once project block (never touched once present),
// without destroying custom content. Returns the new content.
