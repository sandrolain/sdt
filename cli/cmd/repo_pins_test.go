package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The repository declares its toolchains and its CI actions in a few files that
// drift silently: a version copied into a workflow, a tag that floats to a new
// release nobody reviewed. Both were real findings of the consistency review
// (decisions F3 and the accepted GitHub Actions integration), so both become a
// gate rather than a convention an author has to remember.
//
// The tests skip when the file is not in the checkout: they describe this
// repository, not a bootstrapped project.

// repoFile reads a path relative to the repository root, skipping the test when
// the checkout does not carry it.
func repoFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", name)) //#nosec G304 -- fixed repository files
	if err != nil {
		t.Skipf("%s is not in this checkout", name)
	}
	return string(data)
}

// toolVersions parses `.tool-versions` into tool -> version.
func toolVersions(t *testing.T) map[string]string {
	t.Helper()
	versions := map[string]string{}
	for _, line := range strings.Split(repoFile(t, ".tool-versions"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tool, version, found := strings.Cut(line, " ")
		if !found {
			continue
		}
		versions[tool] = strings.TrimSpace(version)
	}
	return versions
}

// TestGoToolchainDeclarationsAgree holds go.mod authoritative for the Go
// version and .tool-versions consistent with it: a contributor's `go` and CI's
// `go-version-file` must resolve the same toolchain, or a build passes on one
// machine and fails on the other.
func TestGoToolchainDeclarationsAgree(t *testing.T) {
	gomod := repoFile(t, "go.mod")
	directive := regexp.MustCompile(`(?m)^go ([0-9]+\.[0-9]+(\.[0-9]+)?)$`).FindStringSubmatch(gomod)
	if directive == nil {
		t.Fatalf("go.mod carries no `go <version>` directive")
	}
	declared := toolVersions(t)["golang"]
	if declared == "" {
		t.Fatalf(".tool-versions declares no golang entry")
	}
	if declared != directive[1] {
		t.Errorf("go.mod requires Go %s but .tool-versions pins %s: the two declarations must name one toolchain", directive[1], declared)
	}
}

// TestCIConsumesDeclaredBunVersion keeps Bun declared once. A `bun-version:`
// input in a workflow is a second declaration that drifts from .tool-versions,
// which is exactly what it did (1.4.4 in CI, 1.4.2 locally).
func TestCIConsumesDeclaredBunVersion(t *testing.T) {
	if toolVersions(t)["bun"] == "" {
		t.Fatalf(".tool-versions declares no bun entry")
	}
	ci := repoFile(t, filepath.Join(".github", "workflows", "ci.yml"))
	if !strings.Contains(ci, "bun-version-file: .tool-versions") {
		t.Errorf("ci.yml does not read the Bun version from .tool-versions (setup-bun `bun-version-file`)")
	}
	if regexp.MustCompile(`(?m)^\s*bun-version:`).MatchString(ci) {
		t.Errorf("ci.yml hardcodes `bun-version:`, duplicating the .tool-versions declaration")
	}
}

// TestWorkflowActionsArePinned requires every `uses:` to name an immutable
// commit with the release its comment names. A floating tag is a dependency
// nobody reviews at the moment it changes; a SHA without a readable version is
// an audit that cannot be read.
func TestWorkflowActionsArePinned(t *testing.T) {
	workflows, err := filepath.Glob(filepath.Join("..", "..", ".github", "workflows", "*.yml"))
	if err != nil || len(workflows) == 0 {
		t.Skip(".github/workflows holds no workflow in this checkout")
	}
	// uses: owner/repo[/subpath]@<40 hex> # vX.Y.Z
	pinned := regexp.MustCompile(`^\s*uses:\s+[A-Za-z0-9._-]+/[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*@[0-9a-f]{40}\s+#\s+v[0-9]+\.[0-9]+\.[0-9]+\S*\s*$`)
	for _, path := range workflows {
		data, err := os.ReadFile(path) //#nosec G304 -- workflow files of this repository
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "uses:") && !strings.HasPrefix(trimmed, "- uses:") {
				continue
			}
			// A local action has no owner/repo and no release to pin.
			value := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "uses:"))
			if strings.HasPrefix(value, "./") {
				continue
			}
			if !pinned.MatchString(strings.Replace(trimmed, "- uses:", "uses:", 1)) {
				t.Errorf("%s:%d: %q is not pinned to a commit SHA with a release comment", filepath.Base(path), i+1, value)
			}
		}
	}
}
