package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/cli/utils/converter"
	"github.com/sandrolain/sdt/cli/utils/crawler"
	"github.com/spf13/cobra"
)

type frontmatterData struct {
	URL         string `yaml:"url"`
	Title       string `yaml:"title"`
	SavedAt     string `yaml:"saved_at"`
	Date        string `yaml:"date,omitempty"`
	Description string `yaml:"description,omitempty"`
	Keywords    string `yaml:"keywords,omitempty"`
	Author      string `yaml:"author,omitempty"`
}

// Output-format values accepted by the global --format flag.
const (
	formatText = "text"
	formatJSON = "json"
	formatYAML = "yaml"
)

func buildFrontmatter(page crawler.Page, savedAt time.Time) string {
	fm := frontmatterData{
		URL:         page.URL,
		Title:       page.Title,
		SavedAt:     savedAt.Format(time.RFC3339),
		Date:        page.Date,
		Description: page.Description,
		Keywords:    page.Keywords,
		Author:      page.Author,
	}

	data, err := yaml.Marshal(fm)
	if err != nil {
		return ""
	}

	return "---\n" + string(data) + "---\n\n"
}

// crawldownMode is the execution shape resolved from the output flags and the
// number of positional URLs.
type crawldownMode int

const (
	modeStdout crawldownMode = iota
	modeFile
	modeCrawl
	modeCapture
)

// resolveCrawldownMode maps the output flags and the URL count to a mode.
// --output selects crawl mode for a single URL and capture mode for several;
// --output-file is the single-URL file case.
func resolveCrawldownMode(outputDir, outputFile string, urlCount int) (crawldownMode, error) {
	if outputDir != "" && outputFile != "" {
		return modeStdout, fmt.Errorf("cannot use both --output and --output-file")
	}

	if outputFile != "" {
		if urlCount != 1 {
			return modeStdout, fmt.Errorf("--output-file accepts exactly one URL, got %d", urlCount)
		}
		return modeFile, nil
	}

	if outputDir != "" {
		if urlCount == 1 {
			return modeCrawl, nil
		}
		return modeCapture, nil
	}

	return modeStdout, nil
}

// crawlOnlyFlags are meaningful only when crawling a site; outside crawl mode
// they are rejected rather than silently ignored.
var crawlOnlyFlags = []string{
	"depth",
	"exclude",
	"allowed-path",
	"allowed-path-regex",
	"follow-external",
	"ignore-robots-txt",
}

// validateCrawldownFlags rejects flags that do not apply to the resolved mode.
func validateCrawldownFlags(cmd *cobra.Command, mode crawldownMode) error {
	if mode != modeCrawl {
		for _, name := range crawlOnlyFlags {
			if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
				return fmt.Errorf("--%s is only valid in crawl mode (--output with one URL)", name)
			}
		}
	}

	if mode != modeCrawl && mode != modeCapture {
		if f := cmd.Flags().Lookup("download-docs"); f != nil && f.Changed {
			return fmt.Errorf("--download-docs is only valid with an output directory (--output)")
		}
	}

	return nil
}

// capturedPage is one fetched page plus the Markdown body, the capture
// timestamp recorded in its frontmatter, the linked documents fetched when
// --download-docs is set, and the crawler's error count.
type capturedPage struct {
	Page      crawler.Page
	SavedAt   string
	Markdown  string
	Documents []crawler.Document
	Errors    int64
}

// captureReceipt is the machine-readable result of a capture, emitted by
// --format json|yaml. Path is set when the page was written to a file or
// directory; Markdown is set when the body went to stdout.
type captureReceipt struct {
	URL      string `json:"url" yaml:"url"`
	Title    string `json:"title" yaml:"title"`
	SavedAt  string `json:"saved_at" yaml:"saved_at"`
	Path     string `json:"path,omitempty" yaml:"path,omitempty"`
	Markdown string `json:"markdown,omitempty" yaml:"markdown,omitempty"`
}

// fetchSinglePage fetches one URL without following page links and returns the
// page plus its Markdown body with frontmatter and injected H1, honouring the
// suppression flags. With downloadDocs its document links are also fetched.
func fetchSinglePage(targetURL, userAgent string, timeout int, downloadDocs, noFrontmatter, noTitle bool, conv *converter.Converter) (capturedPage, error) {
	c, err := crawler.NewCrawler(targetURL, crawler.Options{
		MaxDepth:          1,
		UserAgent:         userAgent,
		SinglePage:        true,
		DownloadDocuments: downloadDocs,
		RequestTimeout:    timeout,
		Silent:            true,
	})
	if err != nil {
		return capturedPage{}, err
	}

	var resultPage *crawler.Page
	c.OnPage(func(page crawler.Page) {
		resultPage = &page
	})

	var documents []crawler.Document
	c.OnDocument(func(doc crawler.Document) {
		documents = append(documents, doc)
	})

	if err := c.Start(); err != nil {
		return capturedPage{}, err
	}

	if resultPage == nil {
		return capturedPage{}, fmt.Errorf("no content received from %s", targetURL)
	}

	markdown, err := conv.Convert(resultPage.Content)
	if err != nil {
		return capturedPage{}, err
	}

	if !noTitle && resultPage.Title != "" {
		trimmed := strings.TrimSpace(markdown)
		if !strings.HasPrefix(trimmed, "# ") {
			markdown = fmt.Sprintf("# %s\n\n%s", resultPage.Title, markdown)
		}
	}

	now := time.Now().UTC()
	if !noFrontmatter {
		markdown = buildFrontmatter(*resultPage, now) + markdown
	}

	return capturedPage{
		Page:      *resultPage,
		SavedAt:   now.Format(time.RFC3339),
		Markdown:  markdown,
		Documents: documents,
		Errors:    c.Errors(),
	}, nil
}

// outputCaptureReceipts writes the capture receipts in the requested --format.
// With collapseSingle a lone receipt renders as an object, otherwise (and
// always for crawl mode) the output is an array.
func outputCaptureReceipts(cmd *cobra.Command, receipts []captureReceipt, collapseSingle bool) {
	var payload any = receipts
	if collapseSingle && len(receipts) == 1 {
		payload = receipts[0]
	}

	switch getFormat(cmd) {
	case formatJSON:
		out, err := json.MarshalIndent(payload, "", "  ")
		exitWithError(cmd, err)
		outputString(cmd, string(out)+"\n")
	case formatYAML:
		out, err := yaml.Marshal(payload)
		exitWithError(cmd, err)
		outputString(cmd, string(out))
	}
}

var crawldownCmd = &cobra.Command{
	Use:   "crawldown <url>...",
	Short: "Download a web page or site as Markdown",
	Long: `Download web pages as Markdown and read them on stdout or keep them in files.

Mode is chosen from the output flags and the number of URLs:
  - stdout (no --output, no --output-file): fetch each URL once and print its
    Markdown to stdout, in argument order.
  - file (--output-file <file>): fetch exactly one URL and write it to <file>.
  - crawl (--output <dir>, one URL): crawl the site from that URL and save each
    page as a separate .md file in <dir>.
  - capture (--output <dir>, several URLs): fetch each URL once and save one
    .md file per URL in <dir>.

Only HTML web pages are captured. JSON/API endpoints, authenticated pages,
JavaScript-only pages and local files are out of scope; git clones and package
registries are not web pages either.

Content output:
  - --format text (default) prints the Markdown body.
  - --format json|yaml prints a capture receipt per URL: an object for one URL,
    an array for several and in crawl mode. Each carries url, title, saved_at
    and either path (written to a file/directory) or markdown (printed).
  - --no-frontmatter omits the YAML frontmatter block; --no-title omits the
    injected "# <title>" H1. Both apply to stdout, file and capture modes.

Streams and exit codes:
  - Content goes to stdout; progress and errors go to stderr, so

      sdt crawldown <url> > page.md

    is safe. --quiet silences informational logs.
  - A failed URL or page does not abort the rest: the successful pages are still
    written and the command exits non-zero when anything failed.

Politeness: --delay <seconds> waits between URLs in a batch and between requests
while crawling; pass it when capturing many pages from one host.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		outputDir := getStringFlag(cmd, "output", false)
		outputFile := getStringFlag(cmd, "output-file", false)
		maxDepth := getIntFlag(cmd, "depth", false)
		excludedPaths := getStringArrayFlag(cmd, "exclude", false)
		allowedPaths := getStringArrayFlag(cmd, "allowed-path", false)
		allowedPathRegexes := getStringArrayFlag(cmd, "allowed-path-regex", false)
		timeout := getIntFlag(cmd, "timeout", false)
		delay := getIntFlag(cmd, "delay", false)
		userAgent := getStringFlag(cmd, "user-agent", false)
		ignoreRobotsTxt := getBoolFlag(cmd, "ignore-robots-txt", false)
		followExternal := getBoolFlag(cmd, "follow-external", false)
		downloadDocs := getBoolFlag(cmd, "download-docs", false)
		noFrontmatter := getBoolFlag(cmd, "no-frontmatter", false)
		noTitle := getBoolFlag(cmd, "no-title", false)

		conv, err := converter.NewConverter(converter.Options{
			BulletListMarker: "-",
			CodeBlockStyle:   "fenced",
			EmDelimiter:      "*",
			StrongDelimiter:  "**",
			LinkStyle:        "inlined",
		})
		exitWithError(cmd, err)

		mode, err := resolveCrawldownMode(outputDir, outputFile, len(args))
		exitWithError(cmd, err)

		if err := validateCrawldownFlags(cmd, mode); err != nil {
			exitWithError(cmd, err)
			return
		}

		// Single-page modes: fetch each URL once and emit Markdown to stdout, to
		// one file, or (capture mode) one file per URL in the output directory.
		if mode != modeCrawl {
			if mode == modeCapture {
				if err := os.MkdirAll(outputDir, 0o750); err != nil {
					exitWithError(cmd, fmt.Errorf("create output directory: %w", err))
					return
				}
			}

			format := getFormat(cmd)
			receipts := make([]captureReceipt, 0, len(args))
			failures := 0

			for i, targetURL := range args {
				if i > 0 && delay > 0 {
					time.Sleep(time.Duration(delay) * time.Second)
				}

				captured, err := fetchSinglePage(targetURL, userAgent, timeout, downloadDocs, noFrontmatter, noTitle, conv)
				if err != nil {
					slog.Warn("capture failed", "url", targetURL, "err", err)
					failures++
					continue
				}

				receipt := captureReceipt{
					URL:     captured.Page.URL,
					Title:   captured.Page.Title,
					SavedAt: captured.SavedAt,
				}

				switch mode {
				case modeStdout:
					if format == formatText {
						if len(receipts) > 0 {
							outputString(cmd, "\n")
						}
						outputString(cmd, captured.Markdown)
					}
					receipt.Markdown = captured.Markdown
				case modeFile:
					if err := os.WriteFile(outputFile, []byte(captured.Markdown), 0o600); err != nil {
						slog.Warn("write output file", "path", outputFile, "err", err)
						failures++
						continue
					}
					receipt.Path = outputFile
				case modeCapture:
					path := filepath.Join(outputDir, converter.GenerateFilename(captured.Page.URL))
					if err := os.WriteFile(path, []byte(captured.Markdown), 0o600); err != nil {
						slog.Warn("write output file", "path", path, "err", err)
						failures++
						continue
					}
					receipt.Path = path

					for _, doc := range captured.Documents {
						docPath := filepath.Join(outputDir, converter.GenerateAssetFilename(doc.URL))
						if err := os.WriteFile(docPath, doc.Body, 0o600); err != nil {
							slog.Warn("write document", "path", docPath, "err", err)
							failures++
						}
					}
				}

				receipts = append(receipts, receipt)
			}

			if format != formatText {
				outputCaptureReceipts(cmd, receipts, true)
			}

			if failures > 0 {
				slog.Error("crawldown completed with failures", "failures", failures, "urls", len(args))
				exit(1)
				return
			}
			return
		}

		targetURL := args[0]

		// Crawl mode: save pages to output directory.
		if err := os.MkdirAll(outputDir, 0o750); err != nil {
			exitWithError(cmd, fmt.Errorf("create output directory: %w", err))
			return
		}

		c, err := crawler.NewCrawler(targetURL, crawler.Options{
			MaxDepth:            maxDepth,
			UserAgent:           userAgent,
			IgnoreRobotsTxt:     ignoreRobotsTxt,
			FollowExternalLinks: followExternal,
			DownloadDocuments:   downloadDocs,
			RequestTimeout:      timeout,
			RequestDelay:        delay,
			ExcludedPaths:       excludedPaths,
			AllowedPaths:        allowedPaths,
			AllowedPathRegexes:  allowedPathRegexes,
			Silent:              getBoolFlag(cmd, "quiet", false),
		})
		exitWithError(cmd, err)

		type pageEntry struct {
			markdown   string
			rawBytes   []byte
			filename   string
			pageURL    string
			title      string
			savedAt    string
			isDocument bool
		}

		urlToFile := make(map[string]string)
		var urlToFileMutex sync.Mutex

		pageData := make(map[string]pageEntry)
		var pageDataMutex sync.Mutex

		pageCount := 0
		var pageCountMutex sync.Mutex

		c.OnPage(func(page crawler.Page) {
			pageCountMutex.Lock()
			pageCount++
			currentCount := pageCount
			pageCountMutex.Unlock()

			slog.Info("crawling page", "count", currentCount, "url", page.URL)

			markdown, err := conv.Convert(page.Content)
			if err != nil {
				slog.Warn("converting page", "url", page.URL, "err", err)
				return
			}

			if page.Title != "" {
				trimmed := strings.TrimSpace(markdown)
				if !strings.HasPrefix(trimmed, "# ") {
					markdown = fmt.Sprintf("# %s\n\n%s", page.Title, markdown)
				}
			}

			filename := converter.GenerateFilename(page.URL)
			normalizedURL := strings.TrimSuffix(page.URL, "/")

			urlToFileMutex.Lock()
			urlToFile[normalizedURL] = filename
			urlToFileMutex.Unlock()

			now := time.Now().UTC()

			pageDataMutex.Lock()
			pageData[normalizedURL] = pageEntry{
				markdown: buildFrontmatter(page, now) + markdown,
				filename: filename,
				pageURL:  page.URL,
				title:    page.Title,
				savedAt:  now.Format(time.RFC3339),
			}
			pageDataMutex.Unlock()
		})

		c.OnDocument(func(doc crawler.Document) {
			pageCountMutex.Lock()
			pageCount++
			currentCount := pageCount
			pageCountMutex.Unlock()

			slog.Info("downloading document", "count", currentCount, "url", doc.URL)

			filename := converter.GenerateAssetFilename(doc.URL)
			normalizedURL := strings.TrimSuffix(doc.URL, "/")

			urlToFileMutex.Lock()
			urlToFile[normalizedURL] = filename
			urlToFileMutex.Unlock()

			pageDataMutex.Lock()
			pageData[normalizedURL] = pageEntry{
				rawBytes:   doc.Body,
				filename:   filename,
				pageURL:    doc.URL,
				savedAt:    time.Now().UTC().Format(time.RFC3339),
				isDocument: true,
			}
			pageDataMutex.Unlock()
		})

		exitWithError(cmd, c.Start())

		pageCountMutex.Lock()
		finalCount := pageCount
		pageCountMutex.Unlock()

		slog.Info("crawled pages, saving files", "count", finalCount)

		pageDataMutex.Lock()
		pageDataCopy := make(map[string]pageEntry, len(pageData))
		for k, v := range pageData {
			pageDataCopy[k] = v
		}
		pageDataMutex.Unlock()

		successCount := 0
		processedCount := 0
		failures := 0
		receipts := make([]captureReceipt, 0, len(pageDataCopy))

		for _, data := range pageDataCopy {
			processedCount++
			slog.Info("processing page", "index", processedCount, "total", len(pageDataCopy), "url", data.pageURL)

			urlToFileMutex.Lock()
			urlToFileCopy := make(map[string]string, len(urlToFile))
			for k, v := range urlToFile {
				urlToFileCopy[k] = v
			}
			urlToFileMutex.Unlock()

			outputPath := filepath.Join(outputDir, data.filename)

			if data.isDocument {
				if err := os.WriteFile(outputPath, data.rawBytes, 0o600); err != nil {
					slog.Warn("saving document", "path", outputPath, "err", err)
					failures++
					continue
				}
			} else {
				markdown := converter.ConvertLinksToLocal(data.markdown, data.pageURL, urlToFileCopy)
				if err := os.WriteFile(outputPath, []byte(markdown), 0o600); err != nil {
					slog.Warn("saving page", "path", outputPath, "err", err)
					failures++
					continue
				}
			}

			slog.Info("saved", "path", outputPath)
			successCount++

			receipts = append(receipts, captureReceipt{
				URL:     data.pageURL,
				Title:   data.title,
				SavedAt: data.savedAt,
				Path:    outputPath,
			})
		}

		slog.Info("successfully saved", "count", successCount, "dir", outputDir, "errors", c.Errors())

		if getFormat(cmd) != formatText {
			sort.Slice(receipts, func(i, j int) bool { return receipts[i].URL < receipts[j].URL })
			outputCaptureReceipts(cmd, receipts, false)
		}

		if len(pageDataCopy) == 0 || failures > 0 {
			slog.Error("crawldown completed with failures",
				"pages", len(pageDataCopy), "save_failures", failures, "crawl_errors", c.Errors())
			exit(1)
		}
	},
}

func init() {
	pf := crawldownCmd.PersistentFlags()
	pf.StringP("output", "o", "", "Output directory: crawl mode for one URL, capture mode for several")
	pf.StringP("output-file", "f", "", "Write the single fetched page to this file (default: stdout)")
	pf.IntP("depth", "d", 2, "Maximum crawl depth (crawl mode only)")
	pf.StringArrayP("exclude", "e", []string{}, "URL path prefixes to exclude from crawling (crawl mode only, repeatable)")
	pf.StringArray("allowed-path", []string{}, "Only crawl URLs whose path starts with this prefix (crawl mode only, repeatable)")
	pf.StringArray("allowed-path-regex", []string{}, "Only crawl URLs whose path matches this regex (crawl mode only, repeatable)")
	pf.IntP("timeout", "t", 60, "Request timeout in seconds")
	pf.Int("delay", 0, "Delay in seconds between captured URLs and between crawl requests")
	pf.String("user-agent", "sdt/1.0", "HTTP user agent for requests")
	pf.Bool("ignore-robots-txt", false, "Ignore robots.txt restrictions (crawl mode only)")
	pf.Bool("follow-external", false, "Follow links to external domains (crawl mode only)")
	pf.Bool("download-docs", false, "Also download linked documents such as PDF, Word, Office and text files (crawl and capture modes)")
	pf.Bool("no-frontmatter", false, "Omit the YAML frontmatter block (stdout, file and capture modes)")
	pf.Bool("no-title", false, "Do not inject the page title as an H1 (stdout, file and capture modes)")
	rootCmd.AddCommand(crawldownCmd)
}
