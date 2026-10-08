package cmd

import "testing"

func TestTaskPhaseProgresses(t *testing.T) {
	content := "---\nkind: tasks\n---\n\n## Phase 1\n\n- [x] a <!-- c1 -->\n- [x] b <!-- c2 -->\n\n## Phase 2\n\n- [ ] c <!-- c3 -->\n- [!] d <!-- c4 -->\n"
	got := taskPhaseProgresses(content)
	if len(got) != 2 {
		t.Fatalf("phases = %d, want 2", len(got))
	}
	if got[0].Done != 2 || got[0].Total != 2 || got[0].Current {
		t.Errorf("phase 1 = %+v, want 2/2 not current", got[0])
	}
	if got[1].Done != 0 || got[1].Total != 2 || !got[1].Current {
		t.Errorf("phase 2 = %+v, want 0/2 current", got[1])
	}
	if len(got[1].Blocked) != 1 || got[1].Blocked[0] != "c4" {
		t.Errorf("phase 2 blocked = %v, want [c4]", got[1].Blocked)
	}
}
