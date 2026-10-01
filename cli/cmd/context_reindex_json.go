package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ctxIndexJSONDoc is one document in the context/index.json projection: the
// queryable counterpart of a context/index.md row. It carries the fields an
// agent filters on, never the full body.
type ctxIndexJSONDoc struct {
	Path       string   `json:"path"`
	Kind       string   `json:"kind,omitempty"`
	Status     string   `json:"status,omitempty"`
	Title      string   `json:"title,omitempty"`
	Objective  string   `json:"objective,omitempty"`
	Topics     []string `json:"topics,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Summary    string   `json:"summary,omitempty"`
}

// ctxIndexJSON is the whole projection emitted to context/index.json.
type ctxIndexJSON struct {
	Documents []ctxIndexJSONDoc `json:"documents"`
}

const ctxIndexJSONPath = "context/index.json"

// buildIndexJSON projects every indexed document into a flat, sorted list.
// It reads the same directories as the Markdown builder so the two projections
// cannot cover different documents.
func buildIndexJSON() (string, error) {
	docs := make([]ctxIndexJSONDoc, 0)
	for _, dir := range ctxIndexDirs {
		files, err := dirFiles(dir)
		if err != nil {
			return "", err
		}
		for _, f := range files {
			//#nosec G304 -- fixed repo path
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			content := string(data)
			values := contextwiki.FrontmatterValues(content)
			rel, err := filepath.Rel(sdtWorkDir, f)
			if err != nil {
				rel = f
			}
			docs = append(docs, ctxIndexJSONDoc{
				Path:       filepath.ToSlash(rel),
				Kind:       firstVal(values, "kind"),
				Status:     firstVal(values, "status"),
				Title:      firstVal(values, "title"),
				Objective:  firstVal(values, "objective"),
				Topics:     ctxJSONList(values["topics"]),
				Categories: ctxJSONList(values["categories"]),
				Summary:    firstVal(values, "summary"),
			})
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })

	out, err := json.MarshalIndent(ctxIndexJSON{Documents: docs}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

func firstVal(values map[string][]string, key string) string {
	if v := values[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// ctxJSONList normalizes a frontmatter list into JSON items. The shared
// contextwiki reader keeps an inline flow list ("[a, b]") as a single raw
// string, so it is split here; a block list is already one item per entry.
func ctxJSONList(items []string) []string {
	if len(items) == 1 {
		raw := strings.TrimSpace(items[0])
		if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
			inner := strings.TrimSpace(raw[1 : len(raw)-1])
			if inner == "" {
				return nil
			}
			out := make([]string, 0)
			for _, p := range strings.Split(inner, ",") {
				if v := strings.Trim(strings.TrimSpace(p), `"'`); v != "" {
					out = append(out, v)
				}
			}
			return out
		}
	}
	return items
}

// writeIndexJSONIfChanged writes context/index.json when its content changed.
func writeIndexJSONIfChanged(content string) (bool, error) {
	if existing, err := os.ReadFile(ctxIndexJSONPath); err == nil && string(existing) == content { //#nosec G304 -- generated index
		return false, nil
	}
	if err := os.WriteFile(ctxIndexJSONPath, []byte(content), 0o644); err != nil { //#nosec G306 -- user work file
		return false, err
	}
	return true, nil
}

// ctxIndexJSONSummary reports the per-index write state for --format json.
type ctxIndexJSONSummary struct {
	Markdown string `json:"markdown"`
	JSON     string `json:"json"`
}

func ctxIndexWriteState(written bool) string {
	if written {
		return statusCreated
	}
	return "unchanged"
}

// ctxIndexJSONDotPath is the gjson path the command help cites as the query
// entry point (kept here so the doc and the schema cannot drift).
const ctxIndexJSONDotPath = "documents.#(objective==\"structured-data-access\")#.path"
