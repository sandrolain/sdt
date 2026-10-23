package search

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ListOptions bounds a directory listing.
type ListOptions struct {
	// Root is the directory to list. Empty means the working directory.
	Root string
	// Depth is how many levels below Root to report. Zero or negative means 1:
	// the entries of Root itself.
	Depth int
	// ProjectRoot, SkipDirs and SkipPaths carry the same meaning as in
	// FindOptions: one scope rule for every verb that walks the tree.
	ProjectRoot string
	SkipDirs    []string
	SkipPaths   []string
	// Max caps the number of entries reported; zero is unlimited.
	Max int
	// Stat fills Size and Modified from the directory entry.
	Stat bool
}

// ListEntry is one row of a listing.
type ListEntry struct {
	Path     string    `json:"path"`
	Dir      bool      `json:"dir"`
	Size     int64     `json:"size,omitzero"`
	Modified time.Time `json:"modified,omitzero"`
}

// ListResult is the listing with the counters a caller needs to know it was
// bounded rather than complete.
type ListResult struct {
	Entries   []ListEntry `json:"entries"`
	Count     int         `json:"count"`
	Truncated bool        `json:"truncated,omitempty"`
}

// List reports the entries under Root down to Depth levels, directories first
// then by path. It is the walk `find` already owns — same root, same prune set,
// same scope declaration — bounded by depth instead of filtered by a glob,
// because "what is in this directory" is the entry step of a session and
// `find`'s path list answers a different question.
func List(opts ListOptions) (ListResult, error) {
	absRoot, err := resolveRoot(opts.Root)
	if err != nil {
		return ListResult{}, err
	}
	st, err := os.Stat(absRoot)
	if err != nil {
		return ListResult{}, err
	}
	if !st.IsDir() {
		return ListResult{}, fmt.Errorf("ls: %s is not a directory", opts.Root)
	}
	depth := opts.Depth
	if depth <= 0 {
		depth = 1
	}
	skip := opts.SkipPaths
	if skip == nil {
		skip = defaultSkipPaths
	}
	names, declared := searchScope(opts.ProjectRoot, opts.SkipDirs)
	entries := make([]ListEntry, 0, 32)
	visit := func(path string, d fs.DirEntry) error {
		rel, rerr := rootRel(absRoot, path)
		if rerr != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		level := strings.Count(rel, "/") + 1
		if level > depth {
			return nil
		}
		entry := ListEntry{Path: rel, Dir: d.IsDir()}
		if opts.Stat {
			if info, ierr := d.Info(); ierr == nil {
				entry.Size = info.Size()
				entry.Modified = info.ModTime()
			}
		}
		entries = append(entries, entry)
		if d.IsDir() && level >= depth {
			// The listing stops here: a deeper walk answers a question the
			// caller did not ask and can cost a whole tree.
			return filepath.SkipDir
		}
		return nil
	}
	if walkErr := walkAll(absRoot, names, pathSet(skip), declared, visit); walkErr != nil {
		return ListResult{}, walkErr
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return entries[i].Path < entries[j].Path
	})
	res := ListResult{Entries: entries, Count: len(entries)}
	if opts.Max > 0 && len(res.Entries) > opts.Max {
		res.Entries = res.Entries[:opts.Max]
		res.Count = len(res.Entries)
		res.Truncated = true
	}
	return res, nil
}
