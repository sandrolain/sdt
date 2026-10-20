package cmd

import (
	"os"
	"strconv"
	"strings"
)

// The session-start digest.
//
// The lifecycle used to open a session by reading the whole index, every
// architecture document and every decision — about seventy thousand words in a
// corpus that had grown. A read that scales with the corpus is not an entry
// point. This file is the bounded replacement: `reindex` writes it beside the
// index, it carries the architecture summaries and the decisions in force, and
// its size is a constant a test defends. Everything else stays reachable as a
// pointer or a query, which is what an index is for.

const (
	// sdtEssentialsFile is the generated digest a session starts from.
	sdtEssentialsFile = "context/essentials.md"
	// ctxDecisionStatusAccepted and ctxDecisionStatusProposed are the two states
	// whose decision is still in force; the rest (rejected, deprecated,
	// superseded) are history and stay out of the digest.
	ctxDecisionStatusAccepted = "accepted"
	ctxDecisionStatusProposed = "proposed"
	// ctxEssentialsWordBudget caps the body. A session start is paid once per
	// session, before any work, so it gets a fixed allowance rather than the
	// corpus size: growth shows up as a failing test, not as a slower first turn.
	ctxEssentialsWordBudget = 1000
)

// buildEssentials renders the digest: one section of architecture summaries and
// one of the decisions still in force, with the overflow named instead of
// silently dropped. It never fails on a missing directory — a fresh project has
// neither.
func buildEssentials() (string, error) {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("kind: index\n")
	b.WriteString("summary: Bounded session-start digest — architecture summaries and the decisions in force\n")
	b.WriteString("_generated: auto\n")
	b.WriteString("---\n\n")
	b.WriteString("## Essentials\n\n")
	b.WriteString("_Managed by `sdt context reindex`, capped at ")
	b.WriteString(strconv.Itoa(ctxEssentialsWordBudget))
	b.WriteString(" words. The full documents are pointers: open one when the work touches it._\n\n")

	sections := []struct {
		name string
		dir  string
		keep func(status string) bool
	}{
		{"Architecture", sdtArchitectureDir, nil},
		{"Decisions in force", sdtDecisionsDir, func(status string) bool {
			return status == ctxDecisionStatusAccepted || status == ctxDecisionStatusProposed
		}},
	}

	// The preamble and the section headings count against the budget too: the
	// cap is on the file a session reads, not on the rows alone. The reserve
	// leaves room for the overflow line.
	used := len(strings.Fields(b.String()))
	limit := ctxEssentialsWordBudget - 20
	dropped := 0
	for _, section := range sections {
		files, err := dirFiles(section.dir)
		if err != nil {
			return "", err
		}
		var rows []string
		for _, f := range files {
			_, _, _, status := ctxDocMeta(f)
			if section.keep != nil && !section.keep(status) {
				continue
			}
			rows = append(rows, ctxIndexLine(section.dir, f))
		}
		if len(rows) == 0 {
			continue
		}
		b.WriteString("### " + section.name + "\n\n")
		kept := 0
		for _, row := range rows {
			words := len(strings.Fields(row))
			if used+words > limit {
				dropped += len(rows) - kept
				break
			}
			b.WriteString(row + "\n")
			used += words
			kept++
		}
		b.WriteString("\n")
	}
	if dropped > 0 {
		b.WriteString(strconv.Itoa(dropped))
		b.WriteString(" more beyond the budget — see `context/index.md` or `sdt context list`.\n")
	}
	return b.String(), nil
}

// writeEssentialsIfChanged skips the write when the digest is byte-identical, so
// `reindex` stays idempotent and a timestamped build does not churn the corpus.
func writeEssentialsIfChanged(content string) (string, error) {
	if data, err := os.ReadFile(sdtEssentialsFile); err == nil && string(data) == content {
		return "skipped", nil
	}
	//#nosec G306 -- user work file
	if err := os.WriteFile(sdtEssentialsFile, []byte(content), 0o644); err != nil {
		return "", err
	}
	return statusWritten, nil
}
