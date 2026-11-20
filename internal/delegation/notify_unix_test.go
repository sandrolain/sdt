//go:build unix

package delegation

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const maxWorkerSocketPathLenForTest = 100

func TestWorkerSocketPathsAreShortAndUnique(t *testing.T) {
	rootA := filepath.Join(t.TempDir(), strings.Repeat("long-project-segment-", 5))
	rootB := filepath.Join(t.TempDir(), strings.Repeat("long-project-segment-", 5))
	dirA := filepath.Join(rootA, ".sdt", "delegation", "w")
	dirB := filepath.Join(rootB, ".sdt", "delegation", "w")
	for _, dir := range []string{dirA, dirB} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	pathA, ownerA := workerSocketPaths(dirA, "task-1")
	pathARepeat, ownerARepeat := workerSocketPaths(dirA, "task-1")
	pathOtherTask, _ := workerSocketPaths(dirA, "task-2")
	pathOtherRoot, _ := workerSocketPaths(dirB, "task-1")
	aliasRoot := filepath.Join(t.TempDir(), "delegation-alias")
	if err := os.Symlink(filepath.Dir(dirA), aliasRoot); err != nil {
		t.Fatal(err)
	}
	pathAlias, _ := workerSocketPaths(filepath.Join(aliasRoot, "w"), "task-1")

	if len(pathA) > maxWorkerSocketPathLenForTest {
		t.Fatalf("socket path length = %d, want <= %d: %s", len(pathA), maxWorkerSocketPathLenForTest, pathA)
	}
	if pathA != pathARepeat || ownerA != ownerARepeat {
		t.Fatal("socket and owner paths are not deterministic")
	}
	if pathA == pathOtherTask || pathA == pathOtherRoot {
		t.Fatal("different project/task identities share a socket path")
	}
	if pathA != pathAlias {
		t.Fatal("canonical project root aliases do not share a socket path")
	}
	if filepath.Dir(ownerA) != dirA {
		t.Fatalf("owner path = %s, want it to remain under %s", ownerA, dirA)
	}
}

func TestWorkerSocketPathsFallBackWhenTempPathIsTooLong(t *testing.T) {
	longTemp := filepath.Join(t.TempDir(), strings.Repeat("long-temp-segment-", 8))
	if err := os.MkdirAll(longTemp, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", longTemp)

	path, _ := workerSocketPaths(filepath.Join(t.TempDir(), ".sdt", "delegation", "w"), "task-1")
	if len(path) > maxWorkerSocketPathLenForTest {
		t.Fatalf("fallback socket path length = %d, want <= %d: %s", len(path), maxWorkerSocketPathLenForTest, path)
	}
	if !strings.HasPrefix(path, "/tmp/") {
		t.Fatalf("socket path = %s, want the short /tmp fallback", path)
	}
}

func TestNotifierCloseRemovesSocketAndOwner(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Create(Task{ID: "task-1", Role: "reviewer", Deadline: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if admitted, err := store.TryAdmit("task-1", map[string]int{"entry": 1}); err != nil || !admitted {
		t.Fatalf("capacity admission = %v, %v", admitted, err)
	}
	token, err := NewWorkerToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordAttemptStarting("task-1", Attempt{ID: "attempt-1"}, token); err != nil {
		t.Fatal(err)
	}
	identity := ProcessIdentity{PID: os.Getpid(), GroupID: os.Getpid(), StartToken: token}
	if err := store.RecordAttemptWorker("task-1", "attempt-1", identity); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notifier, err := store.Listen(ctx, "task-1", identity)
	if err != nil {
		t.Fatal(err)
	}
	path, ownerPath := workerSocketPaths(filepath.Join(store.Root(), "w"), "task-1")
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("worker socket = %v, %v", info, err)
	}
	if _, err := os.Stat(ownerPath); err != nil {
		t.Fatalf("owner marker: %v", err)
	}
	socketDir, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if socketDir.Mode().Perm() != 0o700 {
		t.Fatalf("worker socket directory mode = %04o, want 0700", socketDir.Mode().Perm())
	}
	if err := notifier.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{path, ownerPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("%s remains after close: %v", path, err)
		}
	}
	if err := store.WaitWorkerClosed(context.Background(), "task-1"); err != nil {
		t.Fatalf("WaitWorkerClosed after cleanup: %v", err)
	}
}

func TestRemoveStaleWorkerSocketRemovesSocketAndOwner(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".sdt", "delegation", "w")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path, ownerPath := workerSocketPaths(dir, "task-1")
	if err := ensureWorkerSocketDirectory(path); err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	owner, err := json.Marshal(workerSocketOwner{
		TaskID:   "task-1",
		Identity: ProcessIdentity{PID: 1, GroupID: 1, StartToken: "stale"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ownerPath, owner, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := removeStaleWorkerSocket(path, ownerPath); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{path, ownerPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("%s remains after stale cleanup: %v", path, err)
		}
	}
}
