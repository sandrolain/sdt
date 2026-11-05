package crawler

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxSitemapURLs bounds how many sitemap URLs seed a crawl.
const maxSitemapURLs = 1000

// maxSitemapBytes caps a single sitemap/robots.txt read.
const maxSitemapBytes = 5 << 20

type sitemapURLSet struct {
	URLs []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

type sitemapIndex struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

// discoverSitemapURLs returns the URLs declared by the site's robots.txt
// sitemaps (falling back to /sitemap.xml), with one level of sitemap-index
// recursion, capped at maxSitemapURLs and filtered by the crawler's
// allow/exclude predicates. It is best-effort: any failure returns what was
// found so far, never an error, so the BFS still proceeds.
func discoverSitemapURLs(base *url.URL, userAgent string, timeout time.Duration, allowed func(string) bool) []string {
	client := &http.Client{Timeout: timeout}
	origin := base.Scheme + "://" + base.Host

	locs := robotsSitemaps(client, origin, userAgent)
	if len(locs) == 0 {
		locs = []string{origin + "/sitemap.xml"}
	}

	seen := map[string]bool{}
	var out []string
	for _, loc := range locs {
		if len(out) >= maxSitemapURLs {
			break
		}
		data := fetchSitemap(client, loc, userAgent)
		if data == nil {
			continue
		}
		if children := sitemapIndexChildren(data); len(children) > 0 {
			for _, child := range children {
				if len(out) >= maxSitemapURLs {
					break
				}
				if cd := fetchSitemap(client, child, userAgent); cd != nil {
					appendUnique(&out, seen, sitemapURLs(cd), allowed)
				}
			}
			continue
		}
		appendUnique(&out, seen, sitemapURLs(data), allowed)
	}

	if len(out) > maxSitemapURLs {
		out = out[:maxSitemapURLs]
	}
	return out
}

// robotsSitemaps returns the `Sitemap:` locations declared in robots.txt.
func robotsSitemaps(client *http.Client, origin, userAgent string) []string {
	data := fetchSitemap(client, origin+"/robots.txt", userAgent)
	if data == nil {
		return nil
	}
	const prefix = "sitemap:"
	var locs []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if len(line) >= len(prefix) && strings.EqualFold(line[:len(prefix)], prefix) {
			locs = append(locs, strings.TrimSpace(line[len(prefix):]))
		}
	}
	return locs
}

// fetchSitemap GETs one sitemap/robots URL; nil on any failure.
func fetchSitemap(client *http.Client, u, userAgent string) []byte {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer func() { err = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSitemapBytes))
	if err != nil {
		return nil
	}
	return decodeSitemapBody(u, data)
}

// decodeSitemapBody decompresses a gzipped sitemap and returns any other body
// unchanged. The magic bytes decide, not the file extension, because a sitemap
// URL may be served gzipped under any name (most large sites do).
func decodeSitemapBody(u string, data []byte) []byte {
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		return data
	}

	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		slog.Debug("sitemap: gzip header rejected", "url", u, "err", err)
		return nil
	}

	raw, readErr := io.ReadAll(io.LimitReader(zr, maxSitemapBytes))
	closeErr := zr.Close()
	if readErr != nil {
		slog.Debug("sitemap: gzip body unreadable", "url", u, "err", readErr)
		return nil
	}
	if closeErr != nil {
		slog.Debug("sitemap: gzip trailer rejected", "url", u, "err", closeErr)
		return nil
	}

	return raw
}

// sitemapURLs extracts the `<loc>` URLs from a `<urlset>` document.
func sitemapURLs(data []byte) []string {
	var set sitemapURLSet
	if err := xml.Unmarshal(data, &set); err != nil {
		return nil
	}
	locs := make([]string, 0, len(set.URLs))
	for _, u := range set.URLs {
		locs = append(locs, strings.TrimSpace(u.Loc))
	}
	return locs
}

// sitemapIndexChildren extracts the child sitemap locations from a
// `<sitemapindex>` document.
func sitemapIndexChildren(data []byte) []string {
	var idx sitemapIndex
	if err := xml.Unmarshal(data, &idx); err != nil {
		return nil
	}
	children := make([]string, 0, len(idx.Sitemaps))
	for _, s := range idx.Sitemaps {
		children = append(children, strings.TrimSpace(s.Loc))
	}
	return children
}

// appendUnique appends absolute URLs that pass the path filter and were not
// seen before. A filtered-out URL is reported at debug so --sitemap never
// fetches silently what the user excluded.
func appendUnique(out *[]string, seen map[string]bool, urls []string, allowed func(string) bool) {
	for _, raw := range urls {
		if raw == "" || seen[raw] {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue
		}
		if allowed != nil && !allowed(raw) {
			slog.Debug("sitemap seed skipped by the path filter", "url", raw)
			continue
		}
		seen[raw] = true
		*out = append(*out, raw)
	}
}
