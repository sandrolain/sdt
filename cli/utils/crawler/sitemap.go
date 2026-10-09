package crawler

import (
	"encoding/xml"
	"io"
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
// recursion, capped at maxSitemapURLs. It is best-effort: any failure returns
// what was found so far, never an error, so the BFS still proceeds.
func discoverSitemapURLs(base *url.URL, userAgent string, timeout time.Duration) []string {
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
					appendUnique(&out, seen, sitemapURLs(cd))
				}
			}
			continue
		}
		appendUnique(&out, seen, sitemapURLs(data))
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
	return data
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

// appendUnique appends absolute URLs not already seen.
func appendUnique(out *[]string, seen map[string]bool, urls []string) {
	for _, raw := range urls {
		if raw == "" || seen[raw] {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue
		}
		seen[raw] = true
		*out = append(*out, raw)
	}
}
