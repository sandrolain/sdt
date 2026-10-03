package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/research"
	"github.com/spf13/cobra"
)

// fakeProvider is a network-free SearchProvider for the command tests.
type fakeProvider struct {
	results   []research.SearchResult
	searchErr error
	remaining int
	usageErr  error
	lastQuery string
	lastLimit int
	callCount int
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) Search(_ context.Context, query string, limit int) ([]research.SearchResult, error) {
	f.callCount++
	f.lastQuery = query
	f.lastLimit = limit
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.results, nil
}

func (f *fakeProvider) CreditUsage(_ context.Context) (int, error) {
	if f.usageErr != nil {
		return -1, f.usageErr
	}
	return f.remaining, nil
}

// withFakeProvider swaps the provider factory for the duration of a test.
func withFakeProvider(t *testing.T, f *fakeProvider) {
	t.Helper()
	prev := searchProviderFactory
	searchProviderFactory = func(*cobra.Command) (research.SearchProvider, error) { return f, nil }
	t.Cleanup(func() { searchProviderFactory = prev })
}

// runSearch executes the search command and reports the exit code it triggered
// (the commands exit through the `exit` hook, which panics to stop the flow).
func runSearch(t *testing.T, args ...string) (exited int) {
	t.Helper()
	origExit := exit
	exit = func(code int) {
		exited = code
		panic("exit")
	}
	defer func() {
		exit = origExit
		if r := recover(); r != nil && r != "exit" {
			panic(r)
		}
	}()
	execute(t, researchSearchCmd, nil, args...)
	return exited
}

func TestResearchSearchRequiresNetworkAndConfirm(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	f := &fakeProvider{results: []research.SearchResult{{URL: "https://example.com/a"}}}
	withFakeProvider(t, f)

	if code := runSearch(t, "--run", runID, "--query", "q"); code == 0 {
		t.Fatal("search without --network/--confirm must exit non-zero")
	}
	if f.callCount != 0 {
		t.Errorf("provider was called %d times despite the gate", f.callCount)
	}
}

func TestResearchSearchRecordsSourcesAndCheckpoint(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	f := &fakeProvider{
		results: []research.SearchResult{
			{URL: "https://example.com/a", Title: "A"},
			{URL: "https://example.com/a/", Title: "A dup"},
			{URL: "https://example.com/b", Title: "B"},
			{URL: "", Title: "no url"},
		},
		remaining: 100,
	}
	withFakeProvider(t, f)

	execute(t, researchSearchCmd, nil, "--run", runID, "--limit", "5", "--network", "--confirm")

	r, err := research.Load(root, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sources) != 2 {
		t.Fatalf("sources = %d, want 2 (dedup + no-url skipped): %+v", len(r.Sources), r.Sources)
	}
	if r.CountByStatus(research.StatusDiscovered) != 2 {
		t.Errorf("all new sources must be 'discovered': %+v", r.Sources)
	}
	if r.Checkpoint.Operation != "search" {
		t.Errorf("checkpoint = %q, want search", r.Checkpoint.Operation)
	}
	if r.Budget.MaxResults != 5 || r.Budget.Results != 2 {
		t.Errorf("budget = %+v", r.Budget)
	}
	if f.lastLimit != 5 {
		t.Errorf("provider limit = %d, want 5", f.lastLimit)
	}
}

func TestResearchSearchBudgetAbortsBeforeSpending(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	f := &fakeProvider{remaining: 3}
	withFakeProvider(t, f)

	if code := runSearch(t, "--run", runID, "--query", "q", "--max-credits", "10", "--network", "--confirm"); code == 0 {
		t.Fatal("expected a budget abort")
	}
	if f.callCount != 0 {
		t.Errorf("provider search was called despite the budget abort")
	}
}

func TestResearchSearchPropagatesProviderError(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	f := &fakeProvider{searchErr: context.DeadlineExceeded}
	withFakeProvider(t, f)

	// An exit is triggered; the command must not write a checkpoint.
	if code := runSearch(t, "--run", runID, "--query", "q", "--network", "--confirm"); code == 0 {
		t.Fatal("a provider error must exit non-zero")
	}
}

func TestResearchSearchNoRunFails(t *testing.T) {
	researchTestProject(t)
	f := &fakeProvider{}
	withFakeProvider(t, f)
	if code := runSearch(t, "--query", "q", "--network", "--confirm"); code == 0 {
		t.Fatal("expected an error when no run exists")
	}
}

func TestResearchSearchFindsRunFromNestedDir(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	f := &fakeProvider{results: []research.SearchResult{{URL: "https://example.com/x"}}, remaining: 50}
	withFakeProvider(t, f)

	sub := filepath.Join(root, "nested")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	execute(t, researchSearchCmd, nil, "--network", "--confirm")

	if _, err := research.Load(root, runID); err != nil {
		t.Fatalf("run must resolve from the project root: %v", err)
	}
}

func TestResearchSearchDedupAgainstExisting(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))

	r, err := research.Load(root, runID)
	if err != nil {
		t.Fatal(err)
	}
	r.AddSource(research.Source{CanonicalURL: "https://example.com/a", Status: research.StatusFetched})
	if err := r.Save(root); err != nil {
		t.Fatal(err)
	}

	f := &fakeProvider{results: []research.SearchResult{{URL: "https://example.com/a/"}}, remaining: 50}
	withFakeProvider(t, f)
	execute(t, researchSearchCmd, nil, "--run", runID, "--network", "--confirm")

	r, _ = research.Load(root, runID)
	if len(r.Sources) != 1 {
		t.Fatalf("an existing canonical URL must not be re-added: %+v", r.Sources)
	}
	if r.Sources[0].Status != research.StatusFetched {
		t.Errorf("dedup must not reset an existing source status: %q", r.Sources[0].Status)
	}
}
