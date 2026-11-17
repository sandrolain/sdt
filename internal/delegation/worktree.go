package delegation

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Workspace struct {
	Path            string
	Branch          string
	Commit          string
	SelectedChanges []string
}

// PrepareWorkspace creates a dedicated worktree from an explicit commit and
// applies only the selected dirty paths from the principal checkout.
func PrepareWorkspace(ctx context.Context, projectRoot, taskID, commit string, selectedChanges []string) (Workspace, error) {
	if !safeID.MatchString(taskID) {
		return Workspace{}, fmt.Errorf("invalid task id %q", taskID)
	}
	resolved, err := gitOutput(ctx, projectRoot, nil, "rev-parse", "--verify", commit+"^{commit}")
	if err != nil {
		return Workspace{}, fmt.Errorf("resolve source commit: %w", err)
	}
	resolved = strings.TrimSpace(resolved)
	if resolved == "" {
		return Workspace{}, fmt.Errorf("source commit resolved to an empty revision")
	}
	path := filepath.Join(projectRoot, ".sdt", "delegation", "worktrees", taskID)
	if _, err := os.Lstat(path); err == nil {
		return Workspace{}, fmt.Errorf("workspace already exists for task %q", taskID)
	} else if !os.IsNotExist(err) {
		return Workspace{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Workspace{}, err
	}
	branch := "sdt/delegation/" + taskID
	if _, err := gitOutput(ctx, projectRoot, nil, "worktree", "add", "-b", branch, path, resolved); err != nil {
		return Workspace{}, fmt.Errorf("create task worktree: %w", err)
	}
	ws := Workspace{Path: path, Branch: branch, Commit: resolved}
	fail := func(err error) (Workspace, error) {
		_ = gitRun(context.Background(), projectRoot, nil, "worktree", "remove", "--force", path) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
		_ = gitRun(context.Background(), projectRoot, nil, "branch", "-D", branch)                //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
		return Workspace{}, err
	}
	for _, selected := range selectedChanges {
		clean, err := cleanSelectedPath(selected)
		if err != nil {
			return fail(err)
		}
		literal := ":(literal)" + filepath.ToSlash(clean)
		tracked := gitRun(ctx, projectRoot, nil, "ls-files", "--error-unmatch", "--", literal) == nil
		if tracked {
			diff, err := gitOutput(ctx, projectRoot, nil, "diff", "--binary", "HEAD", "--", literal)
			if err != nil {
				return fail(fmt.Errorf("read selected change %q: %w", clean, err))
			}
			if diff != "" {
				if err := gitRun(ctx, path, strings.NewReader(diff), "apply", "--binary", "-"); err != nil {
					return fail(fmt.Errorf("apply selected change %q: %w", clean, err))
				}
			}
		} else if err := copySelectedUntracked(projectRoot, path, clean); err != nil {
			return fail(err)
		}
		ws.SelectedChanges = append(ws.SelectedChanges, clean)
	}
	return ws, nil
}

func cleanSelectedPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("selected change %q must be project-relative", path)
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("selected change %q escapes the project", path)
	}
	return clean, nil
}

func copySelectedUntracked(projectRoot, workspace, path string) error {
	source := filepath.Join(projectRoot, path)
	if err := rejectSymlinkPath(projectRoot, path, true); err != nil {
		return fmt.Errorf("selected untracked path %q: %w", path, err)
	}
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("selected untracked path %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("selected untracked path %q must be a regular file", path)
	}
	in, err := os.Open(source) //#nosec G304 -- path was validated as project-relative
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck
	target := filepath.Join(workspace, path)
	if err := rejectSymlinkPath(workspace, filepath.Dir(path), false); err != nil {
		return fmt.Errorf("workspace parent for selected path %q: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //#nosec G304 -- target is under the dedicated worktree
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close() //nolint:errcheck
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return syncDirectoryTree(filepath.Dir(target))
}

func rejectSymlinkPath(root, relative string, allowFinalFile bool) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	clean, err := cleanSelectedPath(relative)
	if err != nil && relative != "." {
		return err
	}
	if relative == "." {
		clean = "."
	}
	parts := strings.Split(filepath.Clean(clean), string(filepath.Separator))
	if clean == "." {
		parts = nil
	}
	current := resolvedRoot
	final := filepath.Join(resolvedRoot, filepath.Clean(clean))
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path component %q is a symlink", part)
		}
		if (!allowFinalFile || current != final) && !info.IsDir() {
			return fmt.Errorf("path component %q is not a directory", part)
		}
	}
	return nil
}

func gitOutput(ctx context.Context, dir string, stdin io.Reader, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // Inputs are validated as project-confined paths or fixed process-management arguments.
	cmd.Dir = dir
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func gitRun(ctx context.Context, dir string, stdin io.Reader, args ...string) error {
	_, err := gitOutput(ctx, dir, stdin, args...)
	return err
}
