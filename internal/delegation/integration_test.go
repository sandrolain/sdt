package delegation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyAcceptedIntegrationPreviewExecuteAndRepeat(t *testing.T) {
	store, root, _, base := acceptedIntegrationWorkspace(t, "integration-apply")
	writeTestFile(t, filepath.Join(root, "unrelated.txt"), "principal local work\n")
	gitTest(t, root, "add", "unrelated.txt")

	preview, err := store.PreviewIntegration(context.Background(), "integration-apply", root)
	if err != nil {
		t.Fatal(err)
	}
	if preview.NoChanges || preview.AlreadyApplied || len(preview.Conflicts) != 0 || preview.Digest == "" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	for _, want := range []string{"delete.txt", "new file.txt", "tracked.txt"} {
		if !containsString(preview.Paths, want) {
			t.Errorf("preview paths %v do not include %q", preview.Paths, want)
		}
	}
	if !strings.Contains(preview.Patch, "delegate update") || !strings.Contains(preview.Patch, "new delegate file") {
		t.Fatalf("preview patch omits task content: %s", preview.Patch)
	}

	if _, err := store.ApplyIntegration(context.Background(), "integration-apply", root, "stale-digest"); err == nil {
		t.Fatal("stale preview digest was accepted")
	}
	result, err := store.ApplyIntegration(context.Background(), "integration-apply", root, preview.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if !result.AlreadyApplied {
		t.Fatalf("successful apply was not identified: %+v", result)
	}
	if got := readTestFile(t, filepath.Join(root, "tracked.txt")); got != "delegate update\n" {
		t.Fatalf("tracked result = %q", got)
	}
	if got := readTestFile(t, filepath.Join(root, "new file.txt")); got != "new delegate file\n" {
		t.Fatalf("new file = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "delete.txt")); !os.IsNotExist(err) {
		t.Fatalf("deleted file still exists: %v", err)
	}
	if got := readTestFile(t, filepath.Join(root, "unrelated.txt")); got != "principal local work\n" {
		t.Fatalf("unrelated principal change = %q", got)
	}
	if got := strings.TrimSpace(gitTest(t, root, "diff", "--cached", "--name-only")); got != "unrelated.txt" {
		t.Fatalf("principal index changed: staged paths = %q", got)
	}
	if got := strings.TrimSpace(gitTest(t, root, "rev-parse", "HEAD")); got != base {
		t.Fatalf("integration created a commit: HEAD=%s base=%s", got, base)
	}

	taskBeforeRepeat, err := store.Read("integration-apply")
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := store.ApplyIntegration(context.Background(), "integration-apply", root, preview.Digest)
	if err != nil || !repeated.AlreadyApplied {
		t.Fatalf("repeat apply = %+v, %v", repeated, err)
	}
	task, err := store.Read("integration-apply")
	if err != nil {
		t.Fatal(err)
	}
	if task.Integration == nil || task.Integration.Digest != preview.Digest || len(task.Integration.Paths) != 3 {
		t.Fatalf("integration receipt was not persisted: %+v", task.Integration)
	}
	if task.Revision != taskBeforeRepeat.Revision {
		t.Fatalf("repeat apply appended another integration event: revision %d -> %d", taskBeforeRepeat.Revision, task.Revision)
	}
}

func TestApplyAcceptedIntegrationRefusesOverlappingLocalChanges(t *testing.T) {
	store, root, workspace, _ := acceptedIntegrationWorkspace(t, "integration-conflict")
	writeTestFile(t, filepath.Join(workspace, "tracked.txt"), "delegate update\n")
	writeTestFile(t, filepath.Join(root, "tracked.txt"), "principal update\n")

	preview, err := store.PreviewIntegration(context.Background(), "integration-conflict", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Conflicts) == 0 {
		t.Fatalf("overlapping local change was not reported: %+v", preview)
	}
	if _, err := store.ApplyIntegration(context.Background(), "integration-conflict", root, preview.Digest); err == nil {
		t.Fatal("overlapping local change was applied")
	}
	if got := readTestFile(t, filepath.Join(root, "tracked.txt")); got != "principal update\n" {
		t.Fatalf("principal change was overwritten: %q", got)
	}
}

func TestApplyIntegrationRequiresAcceptedResultAndProjectWorktree(t *testing.T) {
	store, root, _, _ := acceptedIntegrationWorkspace(t, "integration-state")
	if err := store.Update("integration-state", "correction", "task queued for correction", func(task *Task) error {
		task.Status = TaskRunning
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PreviewIntegration(context.Background(), "integration-state", root); err == nil {
		t.Fatal("non-accepted task was previewed for integration")
	}
	if err := store.Update("integration-state", "result_ready", "result awaits review", func(task *Task) error {
		task.Status = TaskResultReady
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.AcceptResult("integration-state", "reviewed"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PreviewIntegration(context.Background(), "integration-state", filepath.Dir(root)); err == nil {
		t.Fatal("task was integrated into a different project root")
	}
}

func TestApplyIntegrationRejectsSymlinkChanges(t *testing.T) {
	store, root, workspace, _ := acceptedIntegrationWorkspace(t, "integration-symlink")
	if err := os.Symlink("outside", filepath.Join(workspace, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := store.PreviewIntegration(context.Background(), "integration-symlink", root); err == nil {
		t.Fatal("symlink change was accepted for integration")
	}
}

func TestApplyIntegrationNoChangesAndInvalidPreview(t *testing.T) {
	store, root, _, _ := acceptedIntegrationWorkspaceMode(t, "integration-empty", true)
	preview, err := store.PreviewIntegration(context.Background(), "integration-empty", root)
	if err != nil || !preview.NoChanges || preview.Digest == "" {
		t.Fatalf("empty preview = %+v, %v", preview, err)
	}
	if _, err := store.ApplyIntegration(context.Background(), "integration-empty", root, preview.Digest); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ id, digest string }{
		{"integration-empty", "short"}, {"../escape", strings.Repeat("0", 64)}, {"integration-empty", strings.Repeat("z", 64)},
	} {
		if _, err := store.ApplyIntegration(context.Background(), test.id, root, test.digest); err == nil {
			t.Errorf("invalid apply (%q, %q) was accepted", test.id, test.digest)
		}
	}
}

func TestAcceptanceRejectsCandidateChangedAfterEvidence(t *testing.T) {
	store, _, workspace, _ := acceptedIntegrationWorkspace(t, "integration-stale-evidence")
	if err := store.Update("integration-stale-evidence", "correction_started", "correction started", func(task *Task) error {
		task.Status = TaskRunning
		task.Attempts = append(task.Attempts, Attempt{ID: "attempt-2", Status: "running"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspace, "tracked.txt"), "corrected candidate\n")
	if err := store.RecordCandidateDigest(context.Background(), "integration-stale-evidence", "attempt-2"); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("integration-stale-evidence", "result_ready", "corrected result awaits review", func(task *Task) error { task.Status = TaskResultReady; return nil }); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspace, "tracked.txt"), "changed after evidence\n")
	if err := store.AcceptResult("integration-stale-evidence", "reviewed"); err == nil || !strings.Contains(err.Error(), "candidate changed") {
		t.Fatalf("changed candidate was accepted: %v", err)
	}
}

func TestApplyIntegrationRecoversAppliedPatchAndRejectsChangedReceipt(t *testing.T) {
	store, root, workspace, _ := acceptedIntegrationWorkspace(t, "integration-recover")
	writeTestFile(t, filepath.Join(workspace, "tracked.txt"), "delegate update\n")
	preview, err := store.PreviewIntegration(context.Background(), "integration-recover", root)
	if err != nil {
		t.Fatal(err)
	}
	if err := gitRun(context.Background(), root, strings.NewReader(preview.Patch), "apply", "--binary", "-"); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.PreviewIntegration(context.Background(), "integration-recover", root)
	if err != nil || !recovered.AlreadyApplied {
		t.Fatalf("applied patch was not recognized: %+v, %v", recovered, err)
	}
	if _, err := store.ApplyIntegration(context.Background(), "integration-recover", root, preview.Digest); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspace, "tracked.txt"), "changed after receipt\n")
	if _, err := store.PreviewIntegration(context.Background(), "integration-recover", root); err == nil {
		t.Fatal("changed task worktree was accepted after integration receipt")
	}
}

func TestApplyIntegrationRejectsTamperedProvenanceAndDestinationSymlink(t *testing.T) {
	store, root, _, _ := acceptedIntegrationWorkspace(t, "integration-provenance")
	if err := store.Update("integration-provenance", "tamper", "invalid recorded branch", func(task *Task) error {
		task.Source.Branch = "other-branch"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PreviewIntegration(context.Background(), "integration-provenance", root); err == nil {
		t.Fatal("unexpected recorded branch was accepted")
	}

	store, root, workspace, _ := acceptedIntegrationWorkspace(t, "integration-destination-link")
	if err := os.MkdirAll(filepath.Join(workspace, "linked"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspace, "linked", "file.txt"), "task data\n")
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := store.PreviewIntegration(context.Background(), "integration-destination-link", root); err == nil {
		t.Fatal("changed task candidate was allowed after acceptance")
	}
}

func TestApplyIntegrationRejectsRecordedPathAndCommitTampering(t *testing.T) {
	for _, mutate := range []struct {
		name string
		edit func(*Task)
	}{
		{"workspace", func(task *Task) { task.Source.Workspace = filepath.Join(task.Source.ProjectRoot, "outside") }},
		{"commit", func(task *Task) { task.Source.Commit = "not-a-commit" }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			store, root, _, _ := acceptedIntegrationWorkspace(t, "integration-tamper")
			if err := store.Update("integration-tamper", "tamper", "invalid source provenance", func(task *Task) error {
				mutate.edit(task)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := store.PreviewIntegration(context.Background(), "integration-tamper", root); err == nil {
				t.Fatalf("tampered %s was accepted", mutate.name)
			}
		})
	}
}

func TestApplyIntegrationRejectsMissingWorktreeAndUnavailableBase(t *testing.T) {
	t.Run("missing worktree", func(t *testing.T) {
		store, root, workspace, _ := acceptedIntegrationWorkspace(t, "integration-missing-worktree")
		if err := os.RemoveAll(workspace); err != nil {
			t.Fatal(err)
		}
		if _, err := store.PreviewIntegration(context.Background(), "integration-missing-worktree", root); err == nil {
			t.Fatal("missing recorded worktree was accepted")
		}
	})
	t.Run("unavailable base", func(t *testing.T) {
		store, root, _, _ := acceptedIntegrationWorkspace(t, "integration-missing-base")
		if err := store.Update("integration-missing-base", "tamper", "unavailable source commit", func(task *Task) error {
			task.Source.Commit = strings.Repeat("0", 40)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := store.PreviewIntegration(context.Background(), "integration-missing-base", root); err == nil {
			t.Fatal("unavailable recorded base commit was accepted")
		}
	})
	t.Run("detached worktree", func(t *testing.T) {
		store, root, workspace, base := acceptedIntegrationWorkspace(t, "integration-detached")
		gitTest(t, workspace, "checkout", "--detach", base)
		if _, err := store.PreviewIntegration(context.Background(), "integration-detached", root); err == nil {
			t.Fatal("detached task worktree was accepted")
		}
	})
}

func TestUniqueIntegrationPaths(t *testing.T) {
	got := uniqueStrings([]string{"a", "a", "b", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unique paths = %v", got)
	}
}

func TestSnapshotIntegrationPatchRequiresTaskStateDirectory(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	writeTestFile(t, filepath.Join(root, "file.txt"), "base\n")
	gitTest(t, root, "add", "file.txt")
	gitTest(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	base := strings.TrimSpace(gitTest(t, root, "rev-parse", "HEAD"))
	store := NewStore(t.TempDir())
	if _, _, err := snapshotIntegrationPatch(context.Background(), store, Task{ID: "missing-state", Source: SourceSnapshot{Commit: base}}, root); err == nil {
		t.Fatal("snapshot without the task state directory was accepted")
	}
}

func TestRecordIntegrationLockedRejectsStaleTaskAndDifferentReceipt(t *testing.T) {
	store, root, workspace, _ := acceptedIntegrationWorkspace(t, "integration-receipt")
	writeTestFile(t, filepath.Join(workspace, "tracked.txt"), "delegate update\n")
	preview, err := store.PreviewIntegration(context.Background(), "integration-receipt", root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update("integration-receipt", "correction", "task no longer accepted", func(task *Task) error {
		task.Status = TaskQueued
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.recordIntegrationLocked("integration-receipt", preview); err == nil {
		t.Fatal("receipt was written for a task that is no longer accepted")
	}

	task, err := store.Read("integration-receipt")
	if err != nil {
		t.Fatal(err)
	}
	task.Status = TaskAccepted
	task.Integration = &IntegrationReceipt{Digest: strings.Repeat("0", 64)}
	if err := store.write(task); err != nil {
		t.Fatal(err)
	}
	if err := store.recordIntegrationLocked("integration-receipt", preview); err == nil {
		t.Fatal("different integration receipt was accepted")
	}
}

func acceptedIntegrationWorkspace(t *testing.T, id string) (*Store, string, string, string) {
	return acceptedIntegrationWorkspaceMode(t, id, false)
}

func acceptedIntegrationWorkspaceMode(t *testing.T, id string, empty bool) (*Store, string, string, string) {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	writeTestFile(t, filepath.Join(root, "tracked.txt"), "base\n")
	writeTestFile(t, filepath.Join(root, "delete.txt"), "remove me\n")
	gitTest(t, root, "add", "tracked.txt", "delete.txt")
	gitTest(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	base := strings.TrimSpace(gitTest(t, root, "rev-parse", "HEAD"))
	workspace, err := PrepareWorkspace(context.Background(), root, id, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	if err := store.Create(Task{ID: id, Role: "backend"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordWorkspace(id, root, workspace, nil, json.RawMessage(`{}`), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if !empty {
		writeTestFile(t, filepath.Join(workspace.Path, "tracked.txt"), "delegate update\n")
		if err := os.Remove(filepath.Join(workspace.Path, "delete.txt")); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(workspace.Path, "new file.txt"), "new delegate file\n")
	}
	if err := store.Update(id, "started", "task started", func(task *Task) error {
		task.Status = TaskRunning
		task.Attempts = []Attempt{{ID: "attempt-1", Status: "running"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordCandidateDigest(context.Background(), id, "attempt-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(id, "result_ready", "result awaits review", func(task *Task) error {
		task.Status = TaskResultReady
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.AcceptResult(id, "reviewed"); err != nil {
		t.Fatal(err)
	}
	return store, root, workspace.Path, base
}
