package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// agentWriteInstructionFile writes one generated instruction module. The module's
// frontmatter and marked body are the template's, so a force refresh adopts the
// template's title and summary too — the defect this writer exists for. Content a
// user added outside the markers (a trigger, a note) survives, as it does in the
// generic generated-file writer.
func agentWriteInstructionFile(path, name, templateBody string, force bool) FileResult {
	res := FileResult{Path: path}
	data, err := os.ReadFile(path) //#nosec G304 -- fixed generated dir
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	if exists && !force {
		res.Status = statusSkipped
		res.Reason = statusReasonExists
		return res
	}
	content := string(data)
	if exists && force && hasSection(content, name) {
		_, sectionBody := contextwiki.SplitFrontmatter(templateBody)
		content = agentReplaceSection(content, name, strings.TrimSpace(sectionBody))
		// The header is generated too: replace it, keep everything after it.
		if templateFM, _ := contextwiki.SplitFrontmatter(templateBody); strings.TrimSpace(templateFM) != "" {
			_, rest := contextwiki.SplitFrontmatter(content)
			content = strings.TrimRight(templateFM, "\n") + "\n\n" + strings.TrimLeft(rest, "\n")
		}
	} else {
		content = agentRenderGenerated(name, templateBody)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //#nosec G301
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //#nosec G703 G306 -- generated path
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	if exists {
		res.Status = statusUpdated
	} else {
		res.Status = statusCreated
	}
	return res
}
