//go:build unix

package delegation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskMessagesAppendFilterAndRejectInvalidParticipants(t *testing.T) {
	store := NewStore(t.TempDir())
	task := Task{
		ID: "message-task", Role: "backend", RequiredCapabilities: []string{"file-read"},
		ConfigSnapshot: []byte(`{"agents":[{"name":"copilot","roles":["backend"],"capabilities":["file-read"]},{"name":"other","roles":["qa"],"capabilities":["file-read"]}]}`),
		Attempts:       []Attempt{{Agent: "opencode"}},
	}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	for _, message := range [][3]string{{"principal", "copilot", "coordinate this review"}, {"opencode", "principal", "the patch is ready"}} {
		if _, err := store.SendMessage(task.ID, message[0], message[1], message[2], ""); err != nil {
			t.Fatal(err)
		}
	}
	all, err := store.ListMessages(task.ID, "")
	if err != nil || len(all) != 2 || all[0].Sequence != 1 || all[1].Sequence != 2 {
		t.Fatalf("all messages = %#v, %v", all, err)
	}
	filtered, err := store.ListMessages(task.ID, "copilot")
	if err != nil || len(filtered) != 1 || filtered[0].To != "copilot" {
		t.Fatalf("filtered messages = %#v, %v", filtered, err)
	}
	for _, tc := range []struct{ from, to, text string }{
		{"", "principal", "text"}, {"principal", "", "text"}, {"principal", "copilot", " "},
		{"principal\nadmin", "copilot", "text"}, {"principal", "copilot", string(make([]byte, (32<<10)+1))},
	} {
		if _, err := store.SendMessage(task.ID, tc.from, tc.to, tc.text, ""); err == nil {
			t.Fatalf("accepted invalid message: %#v", tc)
		}
	}
	if _, err := store.SendMessage(task.ID, "principal", "other", "not eligible", ""); err == nil {
		t.Fatal("accepted an agent outside the task role and capability set")
	}
	if _, err := store.ListMessages(task.ID, "unknown"); err == nil {
		t.Fatal("listed messages as an unknown participant")
	}
	if _, err := store.ListMessages("../outside", ""); err == nil {
		t.Fatal("accepted an unsafe task ID")
	}
}

func TestTaskMessagesValidateArtifactsAndConfineStorage(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Create(Task{ID: "artifact-task", Role: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("artifact-task", "artifact", "artifact retained", func(task *Task) error {
		task.Artifacts = []Artifact{{Name: "result-attempt-1"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SendMessage("artifact-task", "principal", "principal", "see result", "result-attempt-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SendMessage("artifact-task", "principal", "principal", "missing", "missing"); err == nil {
		t.Fatal("accepted an artifact not retained by the task")
	}
	for _, artifact := range []string{"../escape", "bad\nname", string(make([]byte, 1025))} {
		if _, err := store.SendMessage("artifact-task", "principal", "principal", "invalid artifact", artifact); err == nil {
			t.Fatalf("accepted artifact name %q", artifact)
		}
	}
	messagePath := filepath.Join(store.Root(), "tasks", "artifact-task", "messages.jsonl")
	info, err := os.Stat(messagePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("message mode = %o, want 600", info.Mode().Perm())
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.Symlink(outside, filepath.Join(store.Root(), "tasks", "artifact-task", "messages.jsonl-link")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.openTaskRelative(filepath.Join("tasks", "artifact-task", "..", "..", "..", "escape"), os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		t.Fatal("accepted a path escaping task storage")
	}
	if _, err := store.openTaskRelative(filepath.Join("tasks", "artifact-task", "messages.jsonl-link"), os.O_RDONLY, 0); err == nil {
		t.Fatal("followed a symlink in task storage")
	}
}
