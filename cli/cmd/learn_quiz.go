package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/learn"
)

// ── sdt learn quiz (the zero-hint manifest) ───────────────────────────────────
//
// A quiz manifest names the concepts to be quizzed and the evidence an answer
// must cite. It deliberately carries no answer: zero-hint by default, so the
// learner retrieves before the manifest nudges. `--hint` opts into the page
// summary as the hint; the hint is recorded in the manifest so a reader knows
// whether the manifest was helped.

// learnQuizVersion is the manifest schema version.
const learnQuizVersion = 1

// learnQuizItem is one question, taken from the concept's lesson. By default it
// carries the prompt and the claim it rests on, never the expected answer: a quiz
// that shipped its answers would stop being zero-hint. `--with-key` is the one
// opt-in that adds the frozen key and its grading rule.
type learnQuizItem struct {
	ID         string `json:"id" yaml:"id"`
	Type       string `json:"type" yaml:"type"`
	Prompt     string `json:"prompt" yaml:"prompt"`
	Objective  int    `json:"objective" yaml:"objective"`
	Claim      string `json:"claim" yaml:"claim"`
	Expected   string `json:"expected,omitempty" yaml:"expected,omitempty"`
	Rubric     string `json:"rubric,omitempty" yaml:"rubric,omitempty"`
	Grading    string `json:"grading,omitempty" yaml:"grading,omitempty"`
	Provenance string `json:"provenance,omitempty" yaml:"provenance,omitempty"`
	Frozen     string `json:"frozen,omitempty" yaml:"frozen,omitempty"`
}

// learnQuizEntry is one concept to be quizzed, with the evidence an answer cites
// and the questions its lesson already holds (none when no lesson exists).
type learnQuizEntry struct {
	Concept  string          `json:"concept" yaml:"concept"`
	Wiki     string          `json:"wiki" yaml:"wiki"`
	Title    string          `json:"title" yaml:"title"`
	Evidence []string        `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	Hint     string          `json:"hint,omitempty" yaml:"hint,omitempty"`
	Items    []learnQuizItem `json:"items,omitempty" yaml:"items,omitempty"`
	Note     string          `json:"note,omitempty" yaml:"note,omitempty"`
}

// learnQuizManifest is the generated or validated quiz set.
type learnQuizManifest struct {
	Version  int              `json:"version" yaml:"version"`
	Hinted   bool             `json:"hinted" yaml:"hinted"`
	Keyed    bool             `json:"keyed,omitempty" yaml:"keyed,omitempty"`
	Concepts []learnQuizEntry `json:"concepts" yaml:"concepts"`
}

var learnQuizCmd = &cobra.Command{
	Use:   "quiz <concept>...",
	Short: "Generate (or validate) a zero-hint quiz manifest for selected concepts",
	Long: `Build a quiz manifest from the selected concepts, or validate one.

WHEN — a quiz is about to be run, or a manifest must be checked before use.
WHEN NOT — you want the concepts themselves: use sdt learn inspect. A concept with
a lesson carries that lesson's questions; one without a lesson carries evidence only
and says so, and its questions are yours to write.

RETURNS — a manifest (version, hinted and keyed flags, one entry per concept with
its title, evidence and — when the concept has a lesson — its items as
prompt/type/claim) or, with ` + "`--validate <file>`" + `, the validation verdict. It
carries no expected answer: zero-hint by default, and ` + "`--hint`" + ` opts into the
page summary only. ` + "`--with-key`" + ` adds the frozen key, its grading rule and its
provenance, which is what ` + "`sdt learn answer`" + ` grades against and the only way the
key enters a manifest. An unknown concept fails rather than yielding a guess. Reads
only.

EXAMPLE — sdt learn quiz backend/auth --format yaml
          sdt learn quiz backend/auth --with-key --format json
          sdt learn quiz --validate quiz.yaml`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if path := getStringFlag(cmd, "validate", false); path != "" {
			learnQuizValidateRun(cmd, path)
			return
		}
		if len(args) == 0 {
			exitWithError(cmd, fmt.Errorf("at least one concept is required: sdt learn quiz <concept>... "+
				"or --validate <file>"))
			return
		}
		bld := loadLearnWiki(cmd)
		st := loadLearnState(cmd)
		hinted := getBoolFlag(cmd, "hint", false)
		withKey := getBoolFlag(cmd, "with-key", false)
		manifest := learnQuizManifest{Version: learnQuizVersion, Hinted: hinted, Keyed: withKey,
			Concepts: make([]learnQuizEntry, 0, len(args))}
		for _, arg := range args {
			entry, err := learnQuizEntryFor(bld, st, arg, hinted, withKey)
			if err != nil {
				exitWithError(cmd, err)
				return
			}
			manifest.Concepts = append(manifest.Concepts, entry)
		}
		learnEmit(cmd, manifest, func() {
			for i, e := range manifest.Concepts {
				outputString(cmd, fmt.Sprintf("%d. %-26s %s\n", i+1, e.Concept, e.Title))
				if len(e.Evidence) > 0 {
					outputString(cmd, "   evidence: "+strings.Join(e.Evidence, ", ")+"\n")
				}
				for _, it := range e.Items {
					outputString(cmd, fmt.Sprintf("   [%s] %s\n", it.Type, it.Prompt))
				}
				if e.Note != "" {
					outputString(cmd, "   note: "+e.Note+"\n")
				}
				if e.Hint != "" {
					outputString(cmd, "   hint: "+e.Hint+"\n")
				}
			}
			if hinted {
				outputString(cmd, "hint: included (--hint)\n")
			} else {
				outputString(cmd, "hint: "+learnHintNone+" (zero-hint by default)\n")
			}
			if withKey {
				outputString(cmd, "key: included (--with-key)\n")
			} else {
				outputString(cmd, "key: none (--with-key to include the frozen key)\n")
			}
		})
	},
}

// learnQuizEntryFor resolves one requested concept to a manifest entry. The
// argument may be a wiki page id or a recorded concept id.
func learnQuizEntryFor(bld *contextwiki.Builder, st *learn.State, arg string, hinted, withKey bool) (learnQuizEntry, error) {
	page := bld.Resolve(arg)
	if page == nil {
		if rec := learnRecord(st, learn.ConceptID(arg)); rec != nil {
			page = bld.Resolve(rec.Wiki)
		}
	}
	if page == nil {
		return learnQuizEntry{}, fmt.Errorf("no wiki page matches the concept %q: "+
			"the manifest names corpus concepts, never invented ones", arg)
	}
	entry := learnQuizEntry{
		Concept:  learn.ConceptID(page.ID),
		Wiki:     page.ID,
		Title:    page.Title,
		Evidence: page.Sources,
	}
	if hinted {
		entry.Hint = page.Summary
	}
	entry.Items = learnQuizItemsFor(page.ID, withKey)
	if len(entry.Items) == 0 {
		entry.Note = "no lesson for this concept: its questions are yours to write"
	}
	return entry, nil
}

// learnQuizItemsFor reads the questions a concept's lesson already carries. By
// default the expected answer stays behind: the manifest is what the learner is
// asked. `withKey` adds the frozen key, its grading rule and its provenance, which
// is what a grading caller needs and the only way the key enters a manifest.
func learnQuizItemsFor(conceptID string, withKey bool) []learnQuizItem {
	path := learnLessonPath(conceptID)
	data, err := os.ReadFile(path) //#nosec G304 -- path built from the page id
	if err != nil {
		return nil
	}
	created := parseFrontmatterField(string(data), "created")
	items := learnLessonItemsOf(sectionOf(string(data), "Assess"))
	out := make([]learnQuizItem, 0, len(items))
	for _, it := range items {
		item := learnQuizItem{
			ID: it.ID, Type: it.Type, Prompt: it.Prompt,
			Objective: it.Objective, Claim: it.Claim,
		}
		if withKey {
			item.Expected = it.Expected
			item.Rubric = it.Rubric
			item.Grading = it.Grading
			item.Provenance = it.Provenance
			item.Frozen = learnEffectiveFrozen(it.Frozen, created)
		}
		out = append(out, item)
	}
	return out
}

// learnQuizValidateRun checks a manifest on disk: the version, a non-empty and
// unique concept per entry, and the zero-hint guarantee (a manifest that claims
// to be un-hinted must carry no hint).
func learnQuizValidateRun(cmd *cobra.Command, path string) {
	var manifest learnQuizManifest
	data, err := os.ReadFile(path) //#nosec G304 -- the manifest path is the caller's own argument
	if err != nil {
		exitWithError(cmd, fmt.Errorf("read manifest: %w", err))
		return
	}
	if strings.EqualFold(filepath.Ext(path), ".json") {
		err = json.Unmarshal(data, &manifest)
	} else {
		err = yaml.Unmarshal(data, &manifest)
	}
	if err != nil {
		exitWithError(cmd, fmt.Errorf("parse manifest %s: %w", path, err))
		return
	}
	if err := learnQuizValidate(manifest); err != nil {
		exitWithError(cmd, fmt.Errorf("manifest %s: %w", path, err))
		return
	}
	outputString(cmd, fmt.Sprintf("manifest %s: valid (%d concept(s), hint %s, key %s)\n", path,
		len(manifest.Concepts), learnHintWord(manifest.Hinted), learnKeyWord(manifest.Keyed)))
}

// learnKeyWord renders whether a manifest carries the frozen key.
func learnKeyWord(keyed bool) string {
	if keyed {
		return learnHintIncluded
	}
	return learnHintNone
}

// learnQuizValidate returns the first defect in a manifest, or nil.
func learnQuizValidate(m learnQuizManifest) error {
	if m.Version != learnQuizVersion {
		return fmt.Errorf("unsupported version %d (want %d)", m.Version, learnQuizVersion)
	}
	if len(m.Concepts) == 0 {
		return fmt.Errorf("no concepts to quiz")
	}
	seen := map[string]bool{}
	for i, e := range m.Concepts {
		if strings.TrimSpace(e.Concept) == "" {
			return fmt.Errorf("concept %d has no id", i+1)
		}
		if strings.TrimSpace(e.Wiki) == "" {
			return fmt.Errorf("concept %q has no wiki page", e.Concept)
		}
		if seen[e.Concept] {
			return fmt.Errorf("concept %q appears twice", e.Concept)
		}
		seen[e.Concept] = true
		if e.Hint != "" && !m.Hinted {
			return fmt.Errorf("concept %q carries a hint but the manifest claims zero-hint", e.Concept)
		}
		for _, it := range e.Items {
			if strings.TrimSpace(it.Prompt) == "" {
				return fmt.Errorf("concept %q has an item with no prompt", e.Concept)
			}
			if !containsItem(ctxLessonItemTypes, it.Type) {
				return fmt.Errorf("item %q type %q is not one of %s", it.ID, it.Type, strings.Join(ctxLessonItemTypes, ", "))
			}
			if it.Expected != "" && !m.Keyed {
				return fmt.Errorf("item %q carries an answer but the manifest claims no key", it.ID)
			}
			if seen[e.Concept+"/"+it.ID] {
				return fmt.Errorf("concept %q carries item %q twice", e.Concept, it.ID)
			}
			seen[e.Concept+"/"+it.ID] = true
		}
	}
	return nil
}

// learnHintWords renders the hint state of a manifest. The two states are named
// once so the report, the validator and the text footer cannot drift apart.
const (
	learnHintIncluded = "included"
	learnHintNone     = "none"
)

// learnHintWord renders the hint state of a manifest.
func learnHintWord(hinted bool) string {
	if hinted {
		return learnHintIncluded
	}
	return learnHintNone
}

func init() {
	learnQuizCmd.Flags().String("validate", "", "Validate an existing manifest file instead of generating one")
	learnQuizCmd.Flags().Bool("hint", false, "Include the page summary as a hint (zero-hint by default)")
	learnQuizCmd.Flags().Bool("with-key", false, "Include each item's frozen key, grading rule and provenance")
	learnCmd.AddCommand(learnQuizCmd)
}
