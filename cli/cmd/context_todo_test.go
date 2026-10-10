package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/corpus"
	"github.com/sandrolain/sdt/internal/todo"
)

func TestContextTodoLifecycle(t *testing.T) {
	runInTempDir(t)

	execute(t, contextTodoAddCmd, nil, "integrate chatbot", "--source", "chat")
	execute(t, contextTodoAddCmd, nil, "rethink the lint pipeline")

	out := execute(t, contextTodoListCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "integrate-chatbot") || !strings.Contains(string(out), "rethink-the-lint-pipeline") {
		t.Fatalf("list must show both items, got:\n%s", out)
	}

	// The projection is regenerated on add and excluded from the corpus.
	proj := mustReadFile(t, ctxTodoProjectionPath)
	if !strings.Contains(proj, "- [ ] integrate chatbot") || !strings.Contains(proj, "source of truth is context/todo.yaml") {
		t.Fatalf("projection after add:\n%s", proj)
	}
	if !corpus.ExcludedPath(ctxTodoProjectionPath) {
		t.Errorf("%s must be excluded from the corpus", ctxTodoProjectionPath)
	}

	execute(t, contextTodoDoneCmd, nil, "integrate-chatbot")
	proj = mustReadFile(t, ctxTodoProjectionPath)
	if !strings.Contains(proj, "- [x] integrate chatbot") {
		t.Fatalf("projection after done:\n%s", proj)
	}
	reg, err := todo.Load(".")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if i, ok := reg.Find("integrate-chatbot"); !ok || !reg.Items[i].Done {
		t.Fatalf("done must mark the item, got %+v", reg.Items)
	}

	execute(t, contextTodoRemoveCmd, nil, "rethink-the-lint-pipeline")
	reg, _ = todo.Load(".")
	if _, ok := reg.Find("rethink-the-lint-pipeline"); ok {
		t.Fatal("remove must drop the item")
	}
}

func TestContextTodoIDDerivationAndSuffix(t *testing.T) {
	runInTempDir(t)
	execute(t, contextTodoAddCmd, nil, "Integrate the Chatbot!")
	execute(t, contextTodoAddCmd, nil, "Integrate the Chatbot!")
	out := execute(t, contextTodoListCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "integrate-the-chatbot-2") {
		t.Fatalf("a colliding id must get a numeric suffix, got:\n%s", out)
	}
}

func TestContextTodoBulkDoneRemovePrune(t *testing.T) {
	runInTempDir(t)
	execute(t, contextTodoAddCmd, nil, "a", "--source", "s1")
	execute(t, contextTodoAddCmd, nil, "b", "--source", "s1")
	execute(t, contextTodoAddCmd, nil, "c", "--source", "s2")

	execute(t, contextTodoDoneCmd, nil, "--all")
	reg, _ := todo.Load(".")
	for _, it := range reg.Items {
		if !it.Done {
			t.Fatalf("--all must mark every item done, got %+v", reg.Items)
		}
	}

	execute(t, contextTodoPruneCmd, nil, "--source", "s1")
	reg, _ = todo.Load(".")
	if _, ok := reg.Find("a"); ok {
		t.Fatal("prune must remove s1 items")
	}
	if _, ok := reg.Find("c"); !ok {
		t.Fatal("prune must keep other sources")
	}

	execute(t, contextTodoRemoveCmd, nil, "c")
	reg, _ = todo.Load(".")
	if len(reg.Items) != 0 {
		t.Fatalf("remove must drop c, got %+v", reg.Items)
	}
}

func TestContextTodoRejectsUnknownID(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string { return string(execute(t, contextTodoRemoveCmd, nil, "nope")) })
	shouldExitWithCode(t, 1, func() string { return string(execute(t, contextTodoDoneCmd, nil, "nope")) })
}

func TestContextTodoTagAndFilterFlags(t *testing.T) {
	runInTempDir(t)

	execute(t, contextTodoAddCmd, nil, "viewer idea", "--tag", "viewer")
	execute(t, contextTodoAddCmd, nil, "cli idea", "--tag", "UI Kit")
	execute(t, contextTodoAddCmd, nil, "untagged idea")

	// A tag is normalized to a kebab-case slug on add.
	reg, err := todo.Load(".")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if i, ok := reg.Find("cli-idea"); !ok || !reg.Items[i].HasTag("ui-kit") {
		t.Fatalf("a tag must be normalized to a slug, got %+v", reg.Items)
	}

	undone := execute(t, contextTodoListCmd, nil, "--undone")
	if !strings.Contains(string(undone), "viewer-idea") || !strings.Contains(string(undone), "untagged-idea") {
		t.Fatalf("--undone must list the open items, got:\n%s", undone)
	}

	execute(t, contextTodoDoneCmd, nil, "cli-idea")
	done := execute(t, contextTodoListCmd, nil, "--done")
	if !strings.Contains(string(done), "cli-idea") || strings.Contains(string(done), "viewer-idea") {
		t.Fatalf("--done must list only the done item, got:\n%s", done)
	}

	tagged := execute(t, contextTodoListCmd, nil, "--tag", "viewer")
	if !strings.Contains(string(tagged), "viewer-idea") || strings.Contains(string(tagged), "untagged-idea") {
		t.Fatalf("--tag viewer must list only the viewer item, got:\n%s", tagged)
	}

	// --undone and --done are mutually exclusive.
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextTodoListCmd, nil, "--undone", "--done"))
	})

	if proj := mustReadFile(t, ctxTodoProjectionPath); !strings.Contains(proj, "#viewer") {
		t.Fatalf("projection must carry the tag:\n%s", proj)
	}
}
