package delegation

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareWorkspaceUsesCommitAndOnlySelectedChanges(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	writeTestFile(t, filepath.Join(root, "tracked.txt"), "base\n")
	gitTest(t, root, "add", "tracked.txt")
	gitTest(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	commit := strings.TrimSpace(gitTest(t, root, "rev-parse", "HEAD"))
	writeTestFile(t, filepath.Join(root, "tracked.txt"), "selected edit\n")
	writeTestFile(t, filepath.Join(root, "unselected.txt"), "leave behind\n")
	writeTestFile(t, filepath.Join(root, "new file.txt"), "selected untracked\n")

	ws, err := PrepareWorkspace(context.Background(), root, "task-1", commit, []string{"tracked.txt", "new file.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if ws.Commit != commit || ws.Branch != "sdt/delegation/task-1" {
		t.Fatalf("workspace provenance = %+v", ws)
	}
	store := NewStore(root)
	if err := store.Create(Task{ID: "task-1", Role: "backend"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordWorkspace("task-1", root, ws, ws.SelectedChanges, []byte(`{"delegation":{}}`), []byte(`{"default":"ask"}`)); err != nil {
		t.Fatal(err)
	}
	state, err := store.Read("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if state.Source.Commit != commit || state.Source.Branch != ws.Branch || state.Source.Workspace != ws.Path || len(state.Source.SelectedChanges) != 2 || state.Source.ConfigDigest == "" || !strings.Contains(string(state.ConfigSnapshot), `"delegation"`) || !strings.Contains(string(state.PermissionPolicySnapshot), `"default"`) {
		t.Fatalf("workspace provenance was not persisted: %+v", state.Source)
	}
	if got := readTestFile(t, filepath.Join(ws.Path, "tracked.txt")); got != "selected edit\n" {
		t.Fatalf("selected tracked content = %q", got)
	}
	if got := readTestFile(t, filepath.Join(ws.Path, "new file.txt")); got != "selected untracked\n" {
		t.Fatalf("selected untracked content = %q", got)
	}
	if _, err := os.Stat(filepath.Join(ws.Path, "unselected.txt")); !os.IsNotExist(err) {
		t.Fatalf("unselected dirty file leaked into workspace: %v", err)
	}
	if got := readTestFile(t, filepath.Join(root, "tracked.txt")); got != "selected edit\n" {
		t.Fatalf("principal checkout changed: %q", got)
	}
}

func TestPrepareWorkspaceRejectsUnsafeSelectionAndMissingUntracked(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	writeTestFile(t, filepath.Join(root, "tracked.txt"), "base\n")
	gitTest(t, root, "add", "tracked.txt")
	gitTest(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	commit := strings.TrimSpace(gitTest(t, root, "rev-parse", "HEAD"))
	if _, err := PrepareWorkspace(context.Background(), root, "task-escape", commit, []string{"../outside"}); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
	if _, err := PrepareWorkspace(context.Background(), root, "task-missing", commit, []string{"missing.txt"}); err == nil {
		t.Fatal("expected missing selected file to be rejected")
	}
}

func TestPrepareWorkspaceRejectsSelectedPathThroughSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	gitTest(t, root, "init", "-q")
	gitTest(t, root, "add", "linked")
	gitTest(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	commit := strings.TrimSpace(gitTest(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(outside, "payload"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareWorkspace(context.Background(), root, "task-symlink", commit, []string{"linked/payload"}); err == nil {
		t.Fatal("expected selected path through a symlink to fail")
	}
}

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
