package research

import "testing"

func TestPendingFetchAndVerify(t *testing.T) {
	r := fixedRun()
	r.AddSource(Source{CanonicalURL: "https://a", Status: StatusDiscovered})
	r.AddSource(Source{CanonicalURL: "https://b", Status: StatusFetched})
	r.AddSource(Source{CanonicalURL: "https://c", Status: StatusParsed})
	r.AddSource(Source{CanonicalURL: "https://d", Status: StatusVerified})
	r.AddSource(Source{CanonicalURL: "https://e", Status: StatusRejected})

	pending := r.PendingFetch()
	if len(pending) != 1 || pending[0] != "https://a" {
		t.Errorf("PendingFetch = %v, want [https://a]", pending)
	}
	// Fetched + parsed are verifiable; verified/rejected/discovered are not.
	if n := r.PendingVerify(); n != 2 {
		t.Errorf("PendingVerify = %d, want 2 (fetched + parsed)", n)
	}
}

func TestPendingFetchEmptyWhenNothingDiscovered(t *testing.T) {
	r := fixedRun()
	r.AddSource(Source{CanonicalURL: "https://a", Status: StatusVerified})
	if got := r.PendingFetch(); len(got) != 0 {
		t.Errorf("PendingFetch = %v, want empty", got)
	}
}
