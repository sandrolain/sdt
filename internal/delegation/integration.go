package delegation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type IntegrationReceipt struct {
	Digest  string    `json:"digest"`
	Paths   []string  `json:"paths,omitempty"`
	Applied time.Time `json:"applied_at"`
}

type IntegrationPreview struct {
	TaskID          string     `json:"task_id"`
	Status          TaskStatus `json:"status"`
	PlanReference   string     `json:"plan,omitempty"`
	PlanPhase       string     `json:"phase,omitempty"`
	SourceCommit    string     `json:"source_commit"`
	SourceWorkspace string     `json:"source_workspace"`
	Digest          string     `json:"digest"`
	Paths           []string   `json:"paths,omitempty"`
	Conflicts       []string   `json:"conflicts,omitempty"`
	Patch           string     `json:"patch,omitempty"`
	AlreadyApplied  bool       `json:"already_applied,omitempty"`
	NoChanges       bool       `json:"no_changes,omitempty"`
}

// PreviewIntegration snapshots the accepted worktree into a binary-safe Git
// patch, then checks whether it can be applied without touching the principal
// checkout. The temporary index keeps the task worktree's own staging area intact.
func (s *Store) PreviewIntegration(ctx context.Context, id, projectRoot string) (IntegrationPreview, error) {
	task, err := s.Read(id)
	if err != nil {
		return IntegrationPreview{}, err
	}
	if task.Status != TaskAccepted {
		return IntegrationPreview{}, fmt.Errorf("task %q must be accepted before integration", id)
	}
	root, workspace, err := validateIntegrationSource(ctx, task, projectRoot)
	if err != nil {
		return IntegrationPreview{}, err
	}
	probeCtx, cancelProbe := context.WithTimeout(ctx, time.Second)
	probeErr := s.ProbeWorker(probeCtx, id)
	cancelProbe()
	if err := ctx.Err(); err != nil {
		return IntegrationPreview{}, err
	}
	if probeErr == nil {
		return IntegrationPreview{}, fmt.Errorf("task %q still has a live worker", id)
	}
	if err := ensureNoLiveWorkers(task, id); err != nil {
		return IntegrationPreview{}, err
	}
	patch, paths, err := snapshotIntegrationPatch(ctx, s, task, workspace)
	if err != nil {
		return IntegrationPreview{}, err
	}
	digestBytes := sha256.Sum256([]byte(patch))
	preview := IntegrationPreview{
		TaskID: id, Status: task.Status, PlanReference: task.PlanReference, PlanPhase: task.PlanPhase, SourceCommit: task.Source.Commit,
		SourceWorkspace: workspace, Digest: hex.EncodeToString(digestBytes[:]),
		Paths: paths, Patch: patch, NoChanges: patch == "",
	}
	if task.CandidateDigest != "" && task.CandidateDigest != preview.Digest {
		return IntegrationPreview{}, fmt.Errorf("task candidate changed after evidence was produced or accepted")
	}
	if task.Integration != nil {
		if task.Integration.Digest != preview.Digest {
			return IntegrationPreview{}, fmt.Errorf("task worktree changed after its accepted result was integrated")
		}
		preview.AlreadyApplied = true
		return preview, nil
	}
	if preview.NoChanges {
		return preview, nil
	}
	if err := checkIntegrationPaths(root, workspace, paths); err != nil {
		preview.Conflicts = append(preview.Conflicts, err.Error())
	}
	if alreadyApplied, err := gitApplyCheck(ctx, root, patch, true); err == nil && alreadyApplied {
		preview.AlreadyApplied = true
		return preview, nil
	}
	for _, path := range overlappingDirtyPaths(ctx, root, paths) {
		preview.Conflicts = append(preview.Conflicts, "destination has local changes at "+path)
	}
	if len(preview.Conflicts) == 0 {
		if err := gitApplyCheckRun(ctx, root, patch, false); err != nil {
			preview.Conflicts = append(preview.Conflicts, strings.TrimSpace(err.Error()))
		}
	}
	sort.Strings(preview.Conflicts)
	preview.Conflicts = uniqueStrings(preview.Conflicts)
	return preview, nil
}

func ensureNoLiveWorkers(task Task, id string) error {
	if task.Worker != nil {
		stopped, err := WorkerStopped(*task.Worker)
		if err != nil {
			return fmt.Errorf("inspect worker before integration: %w", err)
		}
		if !stopped {
			return fmt.Errorf("task %q still has a live worker", id)
		}
		return nil
	}
	for _, attempt := range task.Attempts {
		_, found, err := attemptWorker(attempt)
		if err != nil {
			return fmt.Errorf("inspect attempt worker before integration: %w", err)
		}
		if found {
			return fmt.Errorf("task %q still has a live attempt worker", id)
		}
	}
	return nil
}

func (s *Store) candidateDigest(ctx context.Context, task Task) (string, error) {
	root, workspace, err := validateIntegrationSource(ctx, task, task.Source.ProjectRoot)
	if err != nil {
		return "", err
	}
	if root == "" || workspace == "" {
		return "", fmt.Errorf("task candidate source is unavailable")
	}
	patch, _, err := snapshotIntegrationPatch(ctx, s, task, workspace)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(patch))
	return hex.EncodeToString(digest[:]), nil
}

// ApplyIntegration applies only the exact patch hash shown by a previous preview.
// It updates the working tree, never the index or commit history.
func (s *Store) ApplyIntegration(ctx context.Context, id, projectRoot, expectedDigest string) (IntegrationPreview, error) {
	if len(expectedDigest) != sha256.Size*2 {
		return IntegrationPreview{}, fmt.Errorf("an integration preview digest is required")
	}
	if _, err := hex.DecodeString(expectedDigest); err != nil {
		return IntegrationPreview{}, fmt.Errorf("invalid integration preview digest")
	}
	if !safeID.MatchString(id) {
		return IntegrationPreview{}, fmt.Errorf("invalid task id %q", id)
	}
	var preview IntegrationPreview
	err := s.WithLock("task-"+id, func() error {
		current, err := s.PreviewIntegration(ctx, id, projectRoot)
		if err != nil {
			return err
		}
		if current.Digest != expectedDigest {
			return fmt.Errorf("accepted task result changed after preview; review the new preview before executing")
		}
		if current.NoChanges {
			preview = current
			return nil
		}
		if current.AlreadyApplied {
			task, err := s.Read(id)
			if err != nil {
				return err
			}
			if task.Integration == nil {
				if err := s.recordIntegrationLocked(id, current); err != nil {
					return err
				}
			}
			preview = current
			return nil
		}
		if len(current.Conflicts) != 0 {
			return fmt.Errorf("integration preview has conflicts: %s", strings.Join(current.Conflicts, "; "))
		}
		if err := gitApplyCheckRun(ctx, projectRoot, current.Patch, false); err != nil {
			return fmt.Errorf("integration preflight failed: %w", err)
		}
		if err := gitRun(ctx, projectRoot, strings.NewReader(current.Patch), "apply", "--binary", "-"); err != nil {
			return fmt.Errorf("apply task patch: %w", err)
		}
		if err := s.recordIntegrationLocked(id, current); err != nil {
			return fmt.Errorf("changes were applied but integration receipt could not be recorded: %w", err)
		}
		current.AlreadyApplied = true
		preview = current
		return nil
	})
	return preview, err
}

// recordIntegrationLocked updates state while ApplyIntegration owns the task lock.
func (s *Store) recordIntegrationLocked(id string, preview IntegrationPreview) error {
	task, err := s.Read(id)
	if err != nil {
		return err
	}
	if task.Status != TaskAccepted {
		return fmt.Errorf("task result is no longer accepted")
	}
	if task.Integration != nil {
		if task.Integration.Digest != preview.Digest {
			return fmt.Errorf("a different integration is already recorded")
		}
		return nil
	}
	task.Revision++
	task.UpdatedAt = time.Now().UTC()
	task.Events = append(task.Events, StateEvent{ID: task.Revision, At: task.UpdatedAt, Type: "principal_integrated", Summary: "Principal applied the accepted task changes to the source checkout"})
	task.Integration = &IntegrationReceipt{Digest: preview.Digest, Paths: append([]string(nil), preview.Paths...), Applied: task.UpdatedAt}
	return s.write(task)
}

func validateIntegrationSource(ctx context.Context, task Task, projectRoot string) (string, string, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", "", err
	}
	storedRoot, err := filepath.Abs(task.Source.ProjectRoot)
	if err != nil || storedRoot != root {
		return "", "", fmt.Errorf("task belongs to a different project checkout")
	}
	expectedWorkspace := filepath.Join(root, ".sdt", "delegation", "worktrees", task.ID)
	workspace, err := filepath.Abs(task.Source.Workspace)
	if err != nil || workspace != expectedWorkspace {
		return "", "", fmt.Errorf("task worktree path does not match its project and task identity")
	}
	if task.Source.Branch != "sdt/delegation/"+task.ID {
		return "", "", fmt.Errorf("task branch does not match its task identity")
	}
	if len(task.Source.Commit) < 40 || len(task.Source.Commit) > 64 {
		return "", "", fmt.Errorf("task source commit is invalid")
	}
	if _, err := hex.DecodeString(task.Source.Commit); err != nil {
		return "", "", fmt.Errorf("task source commit is invalid")
	}
	info, err := os.Lstat(workspace)
	if err != nil {
		return "", "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", "", fmt.Errorf("task worktree is not a regular directory")
	}
	rootTop, err := gitOutput(ctx, root, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", err
	}
	workspaceTop, err := gitOutput(ctx, workspace, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", err
	}
	resolvedRoot, rootErr := filepath.EvalSymlinks(root)
	resolvedWorkspace, workspaceErr := filepath.EvalSymlinks(workspace)
	expectedResolvedWorkspace := filepath.Join(resolvedRoot, ".sdt", "delegation", "worktrees", task.ID)
	if rootErr != nil || workspaceErr != nil || resolvedWorkspace != expectedResolvedWorkspace || filepath.Clean(strings.TrimSpace(rootTop)) != resolvedRoot || filepath.Clean(strings.TrimSpace(workspaceTop)) != resolvedWorkspace {
		return "", "", fmt.Errorf("task or source path is not the expected Git worktree")
	}
	branch, err := gitOutput(ctx, workspace, nil, "branch", "--show-current")
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(branch) != task.Source.Branch {
		return "", "", fmt.Errorf("task worktree is on unexpected branch %q", strings.TrimSpace(branch))
	}
	base, err := gitOutput(ctx, workspace, nil, "rev-parse", "--verify", task.Source.Commit+"^{commit}")
	if err != nil || strings.TrimSpace(base) != task.Source.Commit {
		return "", "", fmt.Errorf("task source commit is unavailable in its worktree")
	}
	return root, workspace, nil
}

func snapshotIntegrationPatch(ctx context.Context, store *Store, task Task, workspace string) (string, []string, error) {
	stateDir := filepath.Join(store.Root(), "tasks", task.ID)
	indexFile, err := os.CreateTemp(stateDir, "integration-index-*")
	if err != nil {
		return "", nil, err
	}
	indexPath := indexFile.Name()
	if err := indexFile.Close(); err != nil {
		_ = os.Remove(indexPath) //nolint:errcheck
		return "", nil, err
	}
	defer os.Remove(indexPath) //nolint:errcheck
	env := []string{"GIT_INDEX_FILE=" + indexPath}
	if err := gitRunEnv(ctx, workspace, env, nil, "read-tree", task.Source.Commit); err != nil {
		return "", nil, err
	}
	if err := gitRunEnv(ctx, workspace, env, nil, "add", "--all", "--", "."); err != nil {
		return "", nil, err
	}
	pathsRaw, err := gitOutputEnv(ctx, workspace, env, nil, "diff", "--cached", "--name-only", "-z", "--no-renames", task.Source.Commit)
	if err != nil {
		return "", nil, err
	}
	paths := make([]string, 0)
	changed := make(map[string]bool)
	for _, raw := range strings.Split(pathsRaw, "\x00") {
		if raw == "" {
			continue
		}
		clean, err := cleanSelectedPath(filepath.FromSlash(raw))
		if err != nil || clean == ".git" || strings.HasPrefix(filepath.ToSlash(clean), ".git/") || clean == ".sdt/delegation" || strings.HasPrefix(filepath.ToSlash(clean), ".sdt/delegation/") {
			return "", nil, fmt.Errorf("task patch contains unsafe path %q", raw)
		}
		path := filepath.ToSlash(clean)
		if !changed[path] {
			paths = append(paths, path)
			changed[path] = true
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return "", nil, nil
	}
	stages, err := gitOutputEnv(ctx, workspace, env, nil, "ls-files", "--stage", "-z")
	if err != nil {
		return "", nil, err
	}
	for _, entry := range strings.Split(stages, "\x00") {
		metaPath := strings.SplitN(entry, "\t", 2)
		if len(metaPath) != 2 || !changed[filepath.ToSlash(metaPath[1])] {
			continue
		}
		fields := strings.Fields(metaPath[0])
		if len(fields) > 0 && fields[0] == "120000" {
			return "", nil, fmt.Errorf("task patch changes symlink %q; integration refuses symlink paths", metaPath[1])
		}
	}
	for _, path := range paths {
		if err := rejectSymlinkPath(workspace, filepath.FromSlash(path), true); err != nil {
			return "", nil, fmt.Errorf("unsafe task path %q: %w", path, err)
		}
	}
	patch, err := gitOutputEnv(ctx, workspace, env, nil, "diff", "--cached", "--binary", "--full-index", "--no-renames", task.Source.Commit)
	if err != nil {
		return "", nil, err
	}
	return patch, paths, nil
}

func checkIntegrationPaths(root, workspace string, paths []string) error {
	for _, path := range paths {
		if err := rejectSymlinkPath(root, filepath.FromSlash(path), true); err != nil {
			return fmt.Errorf("unsafe destination path %q: %w", path, err)
		}
		if err := rejectSymlinkPath(workspace, filepath.FromSlash(path), true); err != nil {
			return fmt.Errorf("unsafe task path %q: %w", path, err)
		}
	}
	return nil
}

func overlappingDirtyPaths(ctx context.Context, root string, paths []string) []string {
	tracked, err := gitOutput(ctx, root, nil, "diff", "--name-only", "-z", "--no-renames", "HEAD")
	if err != nil {
		return []string{"could not inspect destination status: " + err.Error()}
	}
	untracked, err := gitOutput(ctx, root, nil, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return []string{"could not inspect destination untracked files: " + err.Error()}
	}
	changed := make(map[string]bool, len(paths))
	for _, path := range paths {
		changed[filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))] = true
	}
	conflicts := make([]string, 0)
	for _, collection := range []string{tracked, untracked} {
		for _, path := range strings.Split(collection, "\x00") {
			if path != "" && changed[filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))] {
				conflicts = append(conflicts, path)
			}
		}
	}
	sort.Strings(conflicts)
	return uniqueStrings(conflicts)
}

func gitApplyCheck(ctx context.Context, root, patch string, reverse bool) (bool, error) {
	args := []string{"apply", "--check", "--binary"}
	if reverse {
		args = []string{"apply", "--reverse", "--check", "--binary"}
	}
	err := gitRun(ctx, root, strings.NewReader(patch), append(args, "-")...)
	return err == nil, err
}

func gitApplyCheckRun(ctx context.Context, root, patch string, reverse bool) error {
	_, err := gitApplyCheck(ctx, root, patch, reverse)
	return err
}

func gitOutputEnv(ctx context.Context, dir string, extraEnv []string, stdin io.Reader, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //#nosec G204 -- Fixed Git executable and internal argv over provenance-checked task paths; no shell.
	cmd.Dir, cmd.Stdin = dir, stdin
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func gitRunEnv(ctx context.Context, dir string, extraEnv []string, stdin io.Reader, args ...string) error {
	_, err := gitOutputEnv(ctx, dir, extraEnv, stdin, args...)
	return err
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}
