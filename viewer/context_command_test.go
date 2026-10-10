package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const analysisDoc = "context/analysis/20261010-analy-x.md"

// analysisCorpus is a project root holding one closable analysis.
func analysisCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, analysisDoc, "---\nkind: analysis\nstatus: active\n---\n\n# X\n")
	return root
}

// recordingDelegator returns a delegator that records the argv it is handed and
// answers with a fixed result, so the tests assert the delegation, not the CLI.
func recordingDelegator(root string, output string, err error) (*delegator, *[]string) {
	var seen []string
	d := &delegator{
		bin:  "/usr/local/bin/sdt",
		root: root,
		run: func(_ context.Context, _, _ string, args ...string) (string, error) {
			seen = append(seen, args...)
			return output, err
		},
	}
	return d, &seen
}

// postCommand issues the delegation request the way the SPA does.
func postCommand(t *testing.T, s *server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, contextCommandPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleContextCommand(rec, req)
	return rec
}

func TestContextCommandDelegatesToTheCLI(t *testing.T) {
	root := analysisCorpus(t)
	s, err := newServer(root)
	if err != nil {
		t.Fatal(err)
	}
	d, seen := recordingDelegator(root, "ok", nil)
	s.cmd = d

	rec := postCommand(t, s, `{"doc":"`+analysisDoc+`","action":"close"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	// the CLI receives the corpus path, not an absolute one, rooted at the project
	want := []string{"context", "close", analysisDoc}
	if len(*seen) != len(want) {
		t.Fatalf("argv = %v, want %v", *seen, want)
	}
	for i := range want {
		if (*seen)[i] != want[i] {
			t.Fatalf("argv = %v, want %v", *seen, want)
		}
	}
	var res contextCommandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || !res.OK {
		t.Fatalf("body = %s, err = %v", rec.Body, err)
	}
}

func TestContextCommandStatusSetPassesTheValue(t *testing.T) {
	root := analysisCorpus(t)
	s, _ := newServer(root)
	d, seen := recordingDelegator(root, "", nil)
	s.cmd = d

	postCommand(t, s, `{"doc":"`+analysisDoc+`","action":"status-set","value":"draft"}`)
	want := []string{"context", "status", "set", analysisDoc, "--status", "draft"}
	if strings.Join(*seen, " ") != strings.Join(want, " ") {
		t.Fatalf("argv = %v, want %v", *seen, want)
	}
}

func TestContextCommandAllowsOnlyTheClosedActionSet(t *testing.T) {
	root := analysisCorpus(t)
	s, _ := newServer(root)
	d, seen := recordingDelegator(root, "", nil)
	s.cmd = d

	// a free-form verb cannot be smuggled through the action field
	for _, body := range []string{
		`{"doc":"` + analysisDoc + `","action":"rm -rf /"}`,
		`{"doc":"` + analysisDoc + `","action":"context lint --fix"}`,
		`{"doc":"` + analysisDoc + `"}`,
	} {
		if rec := postCommand(t, s, body); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("body %s: status = %d, want 422", body, rec.Code)
		}
	}
	if len(*seen) != 0 {
		t.Fatalf("a refused action still reached the CLI: %v", *seen)
	}
}

func TestContextCommandConfinesTheDocumentToTheCorpus(t *testing.T) {
	root := analysisCorpus(t)
	writeFixture(t, root, "outside.md", "# not corpus\n")
	writeFixture(t, root, "context/refs/ignored.md", "# excluded\n")
	s, _ := newServer(root)
	d, seen := recordingDelegator(root, "", nil)
	s.cmd = d

	for _, doc := range []string{"../outside.md", "outside.md", "/etc/passwd", "context/refs/ignored.md"} {
		rec := postCommand(t, s, `{"doc":"`+doc+`","action":"close"}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("doc %q: status = %d, want 404", doc, rec.Code)
		}
	}
	if len(*seen) != 0 {
		t.Fatalf("an out-of-corpus document reached the CLI: %v", *seen)
	}
}

func TestContextCommandRefusesCLIRefusals(t *testing.T) {
	root := analysisCorpus(t)
	s, _ := newServer(root)
	d, _ := recordingDelegator(root, "cannot close analysis: unfinished task checklist", context.Canceled)
	s.cmd = d

	rec := postCommand(t, s, `{"doc":"`+analysisDoc+`","action":"close"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unfinished task checklist") {
		t.Errorf("the refusal reason is not surfaced: %s", rec.Body)
	}
}

func TestContextCommandRefusesCrossOriginAndNonJSON(t *testing.T) {
	root := analysisCorpus(t)
	s, _ := newServer(root)
	d, seen := recordingDelegator(root, "", nil)
	s.cmd = d

	// a page in another site can forge this one; the Sec-Fetch-Site header says so
	cross := httptest.NewRequest(http.MethodPost, contextCommandPath,
		strings.NewReader(`{"doc":"`+analysisDoc+`","action":"close"}`))
	cross.Header.Set("Content-Type", "application/json")
	cross.Header.Set("Sec-Fetch-Site", "cross-site")
	crossRec := httptest.NewRecorder()
	s.handleContextCommand(crossRec, cross)
	if crossRec.Code != http.StatusForbidden {
		t.Errorf("cross-site status = %d, want 403", crossRec.Code)
	}

	// a foreign Origin with a same-site marker is refused on the header too
	origin := httptest.NewRequest(http.MethodPost, contextCommandPath,
		strings.NewReader(`{"doc":"`+analysisDoc+`","action":"close"}`))
	origin.Header.Set("Content-Type", "application/json")
	origin.Header.Set("Origin", "http://evil.example")
	originRec := httptest.NewRecorder()
	s.handleContextCommand(originRec, origin)
	if originRec.Code != http.StatusForbidden {
		t.Errorf("foreign Origin status = %d, want 403", originRec.Code)
	}

	// a form-encoded POST is a simple request: no preflight, so it must be refused
	form := httptest.NewRequest(http.MethodPost, contextCommandPath,
		strings.NewReader(`{"doc":"x","action":"close"}`))
	form.Header.Set("Content-Type", "text/plain")
	formRec := httptest.NewRecorder()
	s.handleContextCommand(formRec, form)
	if formRec.Code != http.StatusBadRequest {
		t.Errorf("non-JSON status = %d, want 400", formRec.Code)
	}
	if len(*seen) != 0 {
		t.Fatalf("a refused request still reached the CLI: %v", *seen)
	}
}

func TestContextCommandIsRefusedWithoutADelegator(t *testing.T) {
	root := analysisCorpus(t)
	s, _ := newServer(root)
	// no CLI resolved: POST is refused and GET says why, so the SPA disables the
	// control with a reason instead of offering a button that always fails
	req := httptest.NewRequest(http.MethodPost, contextCommandPath,
		strings.NewReader(`{"doc":"`+analysisDoc+`","action":"close"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}

	capRec := httptest.NewRecorder()
	s.mux().ServeHTTP(capRec, httptest.NewRequest(http.MethodGet, contextCommandPath, nil))
	var cap contextCommandCapability
	if err := json.Unmarshal(capRec.Body.Bytes(), &cap); err != nil {
		t.Fatalf("capability body = %s, err = %v", capRec.Body, err)
	}
	if cap.Available || cap.Reason == "" {
		t.Fatalf("capability = %+v, want unavailable with a reason", cap)
	}
}

func TestContextCommandCapabilityIsAvailableWithADelegator(t *testing.T) {
	root := analysisCorpus(t)
	s, _ := newServer(root)
	d, _ := recordingDelegator(root, "", nil)
	s.cmd = d
	rec := httptest.NewRecorder()
	s.handleContextCommand(rec, httptest.NewRequest(http.MethodGet, contextCommandPath, nil))
	var cap contextCommandCapability
	if err := json.Unmarshal(rec.Body.Bytes(), &cap); err != nil || !cap.Available {
		t.Fatalf("capability = %+v, err = %v", cap, err)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "localhost", "::1", "[::1]", ""} {
		if !isLoopbackHost(host) {
			t.Errorf("host %q should be loopback", host)
		}
	}
	for _, host := range []string{"0.0.0.0", "192.168.1.10", "example.com"} {
		if isLoopbackHost(host) {
			t.Errorf("host %q should not be loopback", host)
		}
	}
}

func TestEnableWritesRefusesANonLoopbackBindHost(t *testing.T) {
	root := analysisCorpus(t)
	s, _ := newServer(root)
	s.enableWrites("0.0.0.0", "/usr/local/bin/sdt")
	if s.cmd != nil {
		t.Fatal("a non-loopback bind host must not enable document editing")
	}
}

func TestEnableWritesWithAnExplicitCLI(t *testing.T) {
	root := analysisCorpus(t)
	bin := filepath.Join(t.TempDir(), "sdt")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, _ := newServer(root)
	s.enableWrites("127.0.0.1", bin)
	if s.cmd == nil || s.cmd.bin != bin {
		t.Fatalf("delegator = %+v, want bin %s", s.cmd, bin)
	}
}

func TestResolveSdtBinaryMakesAnExplicitPathAbsolute(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "sdt")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// the mutator runs with Dir set to the served root, so a relative name
	// would be looked up there; an explicit path is resolved first
	t.Chdir(dir)
	got := resolveSdtBinary("sdt", "")
	if !filepath.IsAbs(got) {
		t.Fatalf("resolveSdtBinary = %q, want an absolute path", got)
	}
}

func TestResolveSdtBinaryRejectsAnExplicitMissingPath(t *testing.T) {
	// an explicit path that does not exist disables editing rather than
	// silently falling back to some other CLI (decision 0027, point 5)
	if got := resolveSdtBinary("/definitely/not/here/sdt", ""); got != "" {
		t.Fatalf("resolveSdtBinary = %q, want empty", got)
	}
}

func TestResolveSdtBinaryFindsASiblingBinary(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "sdtviewer")
	if err := os.WriteFile(self, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sdt"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// PATH is emptied so only the sibling can answer
	t.Setenv("PATH", dir)
	if got := resolveSdtBinary("", self); got != filepath.Join(dir, "sdt") {
		t.Fatalf("resolveSdtBinary = %q, want the sibling CLI", got)
	}
}

func TestIsExecutableFileRejectsANonExecutable(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "sdt")
	if err := os.WriteFile(plain, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isExecutableFile(plain) {
		t.Error("a non-executable file must not be resolved as the CLI")
	}
	if isExecutableFile(dir) {
		t.Error("a directory must not be resolved as the CLI")
	}
}
