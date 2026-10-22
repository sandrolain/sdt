package contextwiki

import "testing"

func TestDuplicateFrontmatterKeys(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []DuplicateKey
	}{
		{
			name:  "no duplicate",
			input: "---\nkind: wiki\nid: x\n---\n",
			want:  nil,
		},
		{
			name:  "repeated block key with its lines",
			input: "---\nkind: wiki\nrelations:\n  refers_to:\n    - [[a]]\ntags:\n  - x\nrelations:\n  part_of:\n    - [[b]]\n---\n",
			want:  []DuplicateKey{{Key: "relations", Lines: []int{3, 8}}},
		},
		{
			name:  "repeated scalar",
			input: "---\nid: a\nid: b\n---\n",
			want:  []DuplicateKey{{Key: "id", Lines: []int{2, 3}}},
		},
		{
			name:  "three occurrences list every line",
			input: "---\nsummary: a\nsummary: b\nsummary: c\n---\n",
			want:  []DuplicateKey{{Key: "summary", Lines: []int{2, 3, 4}}},
		},
		{
			name:  "indented keys, list items and comments are not keys",
			input: "---\nrelations:\n  part_of:\n    - [[a]]\n  part_of:\n    - [[b]]\n# tags: commented\ntags:\n  - x\n---\n",
			want:  nil,
		},
		{
			name:  "a key only outside the fences is not counted",
			input: "kind: wiki\nid: x\n",
			want:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DuplicateFrontmatterKeys(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i].Key != tc.want[i].Key {
					t.Errorf("key = %q, want %q", got[i].Key, tc.want[i].Key)
				}
				if len(got[i].Lines) != len(tc.want[i].Lines) {
					t.Fatalf("lines = %v, want %v", got[i].Lines, tc.want[i].Lines)
				}
				for j := range got[i].Lines {
					if got[i].Lines[j] != tc.want[i].Lines[j] {
						t.Errorf("lines = %v, want %v", got[i].Lines, tc.want[i].Lines)
					}
				}
			}
		})
	}
}

// A duplicated block key must not change what the page reads as: the parser keeps
// its first-match behaviour, and the duplicate is the lint's fact to report.
func TestDuplicateFrontmatterKeysLeaveTheParseUnchanged(t *testing.T) {
	content := "---\nkind: wiki\nid: x\nrelations:\n  refers_to:\n    - [[a]]\nrelations:\n  part_of:\n    - [[b]]\n---\nbody\n"
	p := ParseContent(content, "context/wiki/x.md", "x")
	if len(p.Relations) != 1 || len(p.Relations["refers_to"]) != 1 {
		t.Fatalf("parse changed: the first verbs block must survive: %+v", p.Relations)
	}
	if dups := DuplicateFrontmatterKeys(p.Frontmatter); len(dups) != 1 || dups[0].Key != "relations" {
		t.Errorf("duplicates = %+v, want the relations key", dups)
	}
}
