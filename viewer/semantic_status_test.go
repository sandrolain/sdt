package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// statusGET requests /api/semantic/status and decodes the payload.
func statusGET(t *testing.T, h http.Handler) semanticStatus {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/semantic/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out semanticStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestSemanticStatusDisabled covers the default contract: with the branch off
// and no snapshot, both capabilities are false and the endpoint never errors.
func TestSemanticStatusDisabled(t *testing.T) {
	root := makeSearchCorpus(t)
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := statusGET(t, h); got.Enabled || got.Snapshot {
		t.Errorf("disabled branch = %+v, want {false false}", got)
	}
}

// TestSemanticStatusEnabled covers the enabled branch: the model loaded, so the
// search toggle is offered and the snapshot it built is reported present.
func TestSemanticStatusEnabled(t *testing.T) {
	probeViewerSemantic(t)
	root := makeSearchCorpus(t)
	enableViewerSemantic(t, root)
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := statusGET(t, h); !got.Enabled || !got.Snapshot {
		t.Errorf("enabled branch = %+v, want {true true}", got)
	}
}

// TestSemanticStatusSnapshotWithoutBranch covers the independence of the two
// fields: the docs map can have a snapshot while the search branch stays off, so
// Snapshot is true and Enabled false.
func TestSemanticStatusSnapshotWithoutBranch(t *testing.T) {
	root := makeSearchCorpus(t)
	saveGraphSnapshot(t, root, map[string][]float32{"context/wiki/model.md#body": {1, 0}})
	h, err := newHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	got := statusGET(t, h)
	if got.Enabled {
		t.Errorf("branch off must report Enabled false, got %+v", got)
	}
	if !got.Snapshot {
		t.Errorf("persisted snapshot must report Snapshot true, got %+v", got)
	}
}
