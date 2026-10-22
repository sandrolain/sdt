package contextwiki

import "strings"

// DuplicateKey is one top-level frontmatter key that appears more than once, with
// every file line it appears on (1-based, the frontmatter's opening fence counting
// as line 1).
type DuplicateKey struct {
	Key   string
	Lines []int
}

// topLevelKey returns the key a frontmatter line declares, and false for a line
// that declares none: an indented line (a nested key or list item), a blank, a
// comment, or a line without a plain `key:` head.
func topLevelKey(line string) (string, bool) {
	if len(line) != len(strings.TrimLeft(line, " \t")) {
		return "", false
	}
	trim := strings.TrimSpace(line)
	if trim == "" || strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, "- ") || trim == frontmatterDelim {
		return "", false
	}
	colon := strings.Index(line, ":")
	if colon <= 0 {
		return "", false
	}
	key := strings.TrimSpace(line[:colon])
	if key == "" {
		return "", false
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.':
		default:
			return "", false
		}
	}
	return key, true
}

// DuplicateFrontmatterKeys reports every top-level key repeated in a raw
// frontmatter block (the fences included, as Page.Frontmatter carries it). The
// single-value readers (`FrontmatterField`, `ParseFrontmatterMap`, the frontmatter
// list reader) stop at the first occurrence, so a second `relations:` block is
// dropped without a word — this scan is the evidence that it existed. A key that
// appears once is not reported; the parser makes it visible already.
//
// The scan is deliberately text-level and separate from the parser (a lint must
// not make a page unreadable for the graph and the viewer). A quoted key or a key
// containing a space is not recognised, which is the reader's own limit and this
// corpus's shape.
func DuplicateFrontmatterKeys(frontmatter string) []DuplicateKey {
	seen := map[string][]int{}
	var order []string
	fenced := false
	for i, line := range strings.Split(frontmatter, "\n") {
		trim := strings.TrimSpace(line)
		if trim == frontmatterDelim {
			if fenced {
				break // closing fence: the block is over
			}
			fenced = true
			continue
		}
		key, ok := topLevelKey(line)
		if !ok {
			continue
		}
		if _, known := seen[key]; !known {
			order = append(order, key)
		}
		seen[key] = append(seen[key], i+1)
	}
	var dups []DuplicateKey
	for _, key := range order {
		if len(seen[key]) > 1 {
			dups = append(dups, DuplicateKey{Key: key, Lines: seen[key]})
		}
	}
	return dups
}
