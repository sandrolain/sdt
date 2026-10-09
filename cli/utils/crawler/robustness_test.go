package crawler

import (
	"errors"
	"net/http"
	"testing"
)

func TestNormalizeURLStripsFragmentAndLowercasesHost(t *testing.T) {
	if got := normalizeURL("https://Example.COM/Page#section"); got != "https://example.com/Page" {
		t.Errorf("normalizeURL = %q, want https://example.com/Page", got)
	}
	// The same page with different fragments normalizes to one capture.
	if a, b := normalizeURL("https://e.test/a#x"), normalizeURL("https://e.test/a#y"); a != b {
		t.Errorf("fragments not stripped: %q vs %q", a, b)
	}
}

func TestIsTransient(t *testing.T) {
	cases := []struct {
		status int
		err    error
		want   bool
	}{
		{200, errors.New("transport"), true},
		{http.StatusTooManyRequests, nil, true},
		{http.StatusServiceUnavailable, nil, true},
		{http.StatusInternalServerError, nil, true},
		{http.StatusNotFound, nil, false},
		{http.StatusForbidden, nil, false},
		{http.StatusOK, nil, false},
	}
	for _, c := range cases {
		if got := isTransient(c.status, c.err); got != c.want {
			t.Errorf("isTransient(%d, %v) = %v, want %v", c.status, c.err, got, c.want)
		}
	}
}
