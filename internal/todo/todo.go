// Package todo reads and writes the idea inbox register (context/todo.yaml):
// short, annotate-only objectives and ideas captured for future analyses. It is
// the one place the register schema is parsed, so the CLI, the Markdown
// projection and lint cannot drift about what the register accepts.
//
// The register is a plain, versioned YAML file: `items` is a list of mapping
// entries. A missing file yields an empty register (its absence never blocks a
// caller); a malformed file returns an error the caller surfaces. The register
// only annotates: nothing here starts an analysis, a plan or any operation.
package todo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/sandrolain/sdt/internal/ctxvocab"
)

// File is the register path, relative to the project root.
const File = "context/todo.yaml"

// DateLayout is the calendar-date format the register stores.
const DateLayout = "2006-01-02"

// Item is one short idea in the inbox.
type Item struct {
	ID      string `yaml:"id"`
	Text    string `yaml:"text"`
	Created string `yaml:"created"`
	Source  string `yaml:"source,omitempty"`
	Done    bool   `yaml:"done,omitempty"`
}

// Register is the parsed context/todo.yaml.
type Register struct {
	Items []Item `yaml:"items,omitempty"`
}

// Path returns the register path for a project root.
func Path(root string) string {
	return filepath.Join(root, File)
}

// Load reads the register for a project root. A missing file yields an empty
// register; a malformed or invalid file returns an error.
func Load(root string) (*Register, error) {
	path := Path(root)
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo-relative path
	if os.IsNotExist(err) {
		return &Register{}, nil
	}
	if err != nil {
		return nil, err
	}
	var reg Register
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := reg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &reg, nil
}

// Save validates and writes the register atomically (a temp file renamed into
// place). It creates the parent directory when absent.
func Save(root string, reg *Register) error {
	if reg == nil {
		reg = &Register{}
	}
	if err := reg.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(reg)
	if err != nil {
		return err
	}
	path := Path(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //#nosec G301 -- project register dir
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil { //#nosec G306 -- user-editable register
		return err
	}
	return os.Rename(tmp, path)
}

// Validate checks the register invariants: a non-empty kebab-case id unique
// across items, non-empty text, and a required parseable created date.
func (r *Register) Validate() error {
	seen := map[string]struct{}{}
	for i := range r.Items {
		it := &r.Items[i]
		if !ctxvocab.SlugRegexp().MatchString(it.ID) {
			return fmt.Errorf("item id %q must be a kebab-case slug (lowercase letters, digits and '-')", it.ID)
		}
		if _, dup := seen[it.ID]; dup {
			return fmt.Errorf("id %q is already used by another item (ids must be unique)", it.ID)
		}
		seen[it.ID] = struct{}{}
		if strings.TrimSpace(it.Text) == "" {
			return fmt.Errorf("item %q: text is required", it.ID)
		}
		if _, err := time.Parse(DateLayout, it.Created); err != nil {
			return fmt.Errorf("item %q: created %q is not a %s date", it.ID, it.Created, DateLayout)
		}
	}
	return nil
}

// IDFor derives a unique kebab-case id: an explicit id wins, otherwise the slug
// of the text; an empty slug becomes "item"; a collision gets a numeric suffix
// (-2, -3, ...).
func (r *Register) IDFor(text, explicit string) string {
	base := ctxvocab.Slug(explicit)
	if explicit == "" {
		base = ctxvocab.Slug(text)
	}
	if base == "" {
		base = "item"
	}
	if _, ok := r.Find(base); !ok {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if _, ok := r.Find(candidate); !ok {
			return candidate
		}
	}
}

// Add appends an item.
func (r *Register) Add(it Item) {
	r.Items = append(r.Items, it)
}

// Find returns the index of the item with the given id and whether it exists.
func (r *Register) Find(id string) (int, bool) {
	id = ctxvocab.Slug(id)
	for i := range r.Items {
		if r.Items[i].ID == id {
			return i, true
		}
	}
	return -1, false
}

// Remove deletes the item with the given id, reporting whether it existed.
func (r *Register) Remove(id string) bool {
	i, ok := r.Find(id)
	if !ok {
		return false
	}
	r.Items = append(r.Items[:i], r.Items[i+1:]...)
	return true
}

// MarkDone marks the item done (it keeps its place in the list), reporting
// whether it existed.
func (r *Register) MarkDone(id string) bool {
	i, ok := r.Find(id)
	if !ok {
		return false
	}
	r.Items[i].Done = true
	return true
}

// Markdown renders the human-readable projection of the register. It is
// CLI-generated; context/todo.yaml remains the source of truth.
func (r *Register) Markdown() string {
	var b strings.Builder
	b.WriteString("<!-- generated by `sdt context todo`; the source of truth is context/todo.yaml -->\n\n")
	b.WriteString("## TODO\n\n")
	if len(r.Items) == 0 {
		b.WriteString("_empty_\n")
		return b.String()
	}
	for _, it := range r.Items {
		mark := " "
		if it.Done {
			mark = "x"
		}
		fmt.Fprintf(&b, "- [%s] %s\n", mark, it.Text)
	}
	return b.String()
}

// Today returns the current UTC calendar day as a YYYY-MM-DD string.
func Today() string {
	return time.Now().UTC().Format(DateLayout)
}
