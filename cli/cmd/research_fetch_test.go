package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/research"
)

// pageServer serves simple HTML pages for the fetch tests.
func pageServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Page A</title></head><body><main><h1>A</h1><p>Alpha body.</p></main></body></html>`))
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Page B</title></head><body><main><h1>B</h1><p>Beta body.</p></main></body></html>`))
	})
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestResearchFetchRecordsProvenanceAndCheckpoint(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)

	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/a", ts.URL+"/b")

	r, err := research.Load(root, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sources) != 2 {
		t.Fatalf("sources = %d, want 2: %+v", len(r.Sources), r.Sources)
	}
	if r.CountByStatus(research.StatusFetched) != 2 {
		t.Errorf("all sources must be 'fetched': %+v", r.Sources)
	}
	if r.Checkpoint.Operation != "fetch" {
		t.Errorf("checkpoint = %q, want fetch", r.Checkpoint.Operation)
	}

	for _, s := range r.Sources {
		if s.SHA256 == "" || s.RawPath == "" || s.Bytes == 0 {
			t.Errorf("source missing provenance: %+v", s)
		}
		abs := filepath.Join(research.RunDir(root, runID), filepath.FromSlash(s.RawPath))
		data, rerr := os.ReadFile(abs)
		if rerr != nil {
			t.Fatalf("raw file %s not written: %v", abs, rerr)
		}
		// The recorded hash must match the bytes on disk.
		sum, size, herr := research.HashFile(abs)
		if herr != nil {
			t.Fatal(herr)
		}
		if sum != s.SHA256 {
			t.Errorf("hash mismatch for %s: manifest %s disk %s", s.CanonicalURL, s.SHA256, sum)
		}
		if size != s.Bytes {
			t.Errorf("bytes mismatch for %s: manifest %d disk %d", s.CanonicalURL, s.Bytes, size)
		}
		if !strings.Contains(strings.ToLower(string(data)), "body") {
			t.Errorf("raw payload looks empty for %s: %q", s.CanonicalURL, string(data))
		}
	}
}

func TestResearchFetchFetchesDiscoveredSources(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)

	// Seed discovered sources via the model (search is tested elsewhere).
	r, err := research.Load(root, runID)
	if err != nil {
		t.Fatal(err)
	}
	r.AddSource(research.Source{CanonicalURL: ts.URL + "/a", Status: research.StatusDiscovered})
	r.AddSource(research.Source{CanonicalURL: ts.URL + "/b", Status: research.StatusVerified}) // must be skipped
	if err := r.Save(root); err != nil {
		t.Fatal(err)
	}

	execute(t, researchFetchCmd, nil, "--run", runID)

	r, _ = research.Load(root, runID)
	if r.CountByStatus(research.StatusFetched) != 1 {
		t.Errorf("only the discovered source should advance to fetched: %+v", r.Sources)
	}
	if r.CountByStatus(research.StatusVerified) != 1 {
		t.Errorf("a verified source must not be re-fetched: %+v", r.Sources)
	}
}

func TestResearchFetchRecordsFailureAndExitsNonZero(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)

	code := runFetch(t, "--run", runID, ts.URL+"/a", ts.URL+"/missing")
	if code == 0 {
		t.Fatal("a failed fetch must exit non-zero")
	}

	r, err := research.Load(root, runID)
	if err != nil {
		t.Fatal(err)
	}
	if r.CountByStatus(research.StatusFetched) != 1 {
		t.Errorf("the successful fetch must still be recorded: %+v", r.Sources)
	}
	// The failed one keeps a retry count and an error and stays retryable.
	failed := r.Source(ts.URL + "/missing")
	if failed == nil || failed.Retries != 1 || failed.Error == "" {
		t.Errorf("failure not recorded: %+v", failed)
	}
	if failed.Status != research.StatusDiscovered {
		t.Errorf("a failed source must stay 'discovered' for resume, got %q", failed.Status)
	}
}

func TestResearchFetchNothingToDo(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	if code := runFetch(t, "--run", runID); code == 0 {
		t.Fatal("fetch with no targets must exit non-zero")
	}
}

func TestResearchFetchDedupByName(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)

	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/a", ts.URL+"/a")

	r, _ := research.Load(root, runID)
	if len(r.Sources) != 1 {
		t.Fatalf("the same URL twice must record one source: %+v", r.Sources)
	}
}

// runFetch executes the fetch command and reports the exit code it triggered.
func runFetch(t *testing.T, args ...string) (exited int) {
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
	execute(t, researchFetchCmd, nil, args...)
	return exited
}
