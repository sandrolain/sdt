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

func TestContextTodoRejectsUnknownID(t *testing.T) {
	runInTempDir(t)
	shouldExitWithCode(t, 1, func() string { return string(execute(t, contextTodoRemoveCmd, nil, "nope")) })
	shouldExitWithCode(t, 1, func() string { return string(execute(t, contextTodoDoneCmd, nil, "nope")) })
}
