package crawler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestDiscoverSitemapURLsFromRobots(t *testing.T) {
	var base string
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("User-agent: *\nSitemap: " + base + "/sitemap.xml\n"))
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><urlset>` +
			"<url><loc>" + base + "/a</loc></url>" +
			"<url><loc>" + base + "/b</loc></url>" +
			"</urlset>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL

	u, _ := url.Parse(srv.URL)
	got := discoverSitemapURLs(u, "sdt-test", 5*time.Second)
	if len(got) != 2 || got[0] != base+"/a" || got[1] != base+"/b" {
		t.Errorf("discoverSitemapURLs = %v, want [%s/a %s/b]", got, base, base)
	}
}

func TestDiscoverSitemapIndexRecursion(t *testing.T) {
	var base string
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><sitemapindex>` +
			"<sitemap><loc>" + base + "/child.xml</loc></sitemap>" +
			"</sitemapindex>"))
	})
	mux.HandleFunc("/child.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><urlset>` +
			"<url><loc>" + base + "/x</loc></url>" +
			"</urlset>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL

	u, _ := url.Parse(srv.URL)
	got := discoverSitemapURLs(u, "sdt-test", 5*time.Second)
	if len(got) != 1 || got[0] != base+"/x" {
		t.Errorf("index recursion = %v, want [%s/x]", got, base)
	}
}
