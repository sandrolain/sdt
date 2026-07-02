package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func buildFrontmatter(page crawler.Page) string {
	fm := frontmatterData{
		URL:         page.URL,
		Title:       page.Title,
		SavedAt:     time.Now().UTC().Format(time.RFC3339),
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

// writeInfo writes a formatted message to w, ignoring the error (non-fatal output).
func writeInfo(w io.Writer, format string, args ...any) {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		_ = err
	}
}

var crawldownCmd = &cobra.Command{
	Use:   "crawldown <url>",
	Short: "Download a web page or site as Markdown",
	Long: `Download a web page or entire website and convert the content to Markdown.

Without --output, fetches the given URL as a single page and prints the Markdown to stdout
(or to a file when --output-file is specified).
With --output <dir>, crawls the full site starting from the given URL and saves each page
as a separate .md file in the output directory.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		targetURL := args[0]
		outputDir := getStringFlag(cmd, "output", false)
		outputFile := getStringFlag(cmd, "output-file", false)
		maxDepth := getIntFlag(cmd, "depth", false)
		excludedPaths := getStringArrayFlag(cmd, "exclude", false)
		allowedPaths := getStringArrayFlag(cmd, "allowed-path", false)
		timeout := getIntFlag(cmd, "timeout", false)
		delay := getIntFlag(cmd, "delay", false)
		userAgent := getStringFlag(cmd, "user-agent", false)
		ignoreRobotsTxt := getBoolFlag(cmd, "ignore-robots-txt", false)
		followExternal := getBoolFlag(cmd, "follow-external", false)
		downloadDocs := getBoolFlag(cmd, "download-docs", false)

		conv, err := converter.NewConverter(converter.Options{
			BulletListMarker: "-",
			CodeBlockStyle:   "fenced",
			EmDelimiter:      "*",
			StrongDelimiter:  "**",
			LinkStyle:        "inlined",
		})
		exitWithError(cmd, err)

		if outputDir != "" && outputFile != "" {
			exitWithError(cmd, fmt.Errorf("cannot use both --output and --output-file"))
			return
		}

		if outputDir == "" {
			// Single-page mode: fetch URL and output markdown to stdout or a file.
			c, err := crawler.NewCrawler(targetURL, crawler.Options{
				MaxDepth:       1,
				UserAgent:      userAgent,
				SinglePage:     true,
				RequestTimeout: timeout,
				Silent:         true,
			})
			exitWithError(cmd, err)

			var resultPage *crawler.Page
			c.OnPage(func(page crawler.Page) {
				resultPage = &page
			})

			exitWithError(cmd, c.Start())

			if resultPage == nil {
				exitWithError(cmd, fmt.Errorf("no content received from %s", targetURL))
				return
			}

			markdown, err := conv.Convert(resultPage.Content)
			exitWithError(cmd, err)

			if resultPage.Title != "" {
				trimmed := strings.TrimSpace(markdown)
				if !strings.HasPrefix(trimmed, "# ") {
					markdown = fmt.Sprintf("# %s\n\n%s", resultPage.Title, markdown)
				}
			}

			markdown = buildFrontmatter(*resultPage) + markdown

			if outputFile != "" {
				if err := os.WriteFile(outputFile, []byte(markdown), 0o600); err != nil {
					exitWithError(cmd, fmt.Errorf("write output file: %w", err))
					return
				}
			} else {
				outputString(cmd, markdown)
			}
			return
		}

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
		})
		exitWithError(cmd, err)

		type pageEntry struct {
			markdown   string
			rawBytes   []byte
			filename   string
			pageURL    string
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

			writeInfo(cmd.OutOrStdout(), "[%d] Crawling: %s\n", currentCount, page.URL)

			markdown, err := conv.Convert(page.Content)
			if err != nil {
				writeInfo(cmd.ErrOrStderr(), "  Error converting page: %v\n", err)
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

			pageDataMutex.Lock()
			pageData[normalizedURL] = pageEntry{
				markdown: buildFrontmatter(page) + markdown,
				filename: filename,
				pageURL:  page.URL,
			}
			pageDataMutex.Unlock()
		})

		c.OnDocument(func(doc crawler.Document) {
			pageCountMutex.Lock()
			pageCount++
			currentCount := pageCount
			pageCountMutex.Unlock()

			writeInfo(cmd.OutOrStdout(), "[%d] Downloading: %s\n", currentCount, doc.URL)

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
				isDocument: true,
			}
			pageDataMutex.Unlock()
		})

		exitWithError(cmd, c.Start())

		pageCountMutex.Lock()
		finalCount := pageCount
		pageCountMutex.Unlock()

		writeInfo(cmd.OutOrStdout(), "\nCrawled %d pages. Saving files...\n\n", finalCount)

		pageDataMutex.Lock()
		pageDataCopy := make(map[string]pageEntry, len(pageData))
		for k, v := range pageData {
			pageDataCopy[k] = v
		}
		pageDataMutex.Unlock()

		successCount := 0
		processedCount := 0

		for _, data := range pageDataCopy {
			processedCount++
			writeInfo(cmd.OutOrStdout(), "[%d/%d] Processing: %s\n", processedCount, len(pageDataCopy), data.pageURL)

			urlToFileMutex.Lock()
			urlToFileCopy := make(map[string]string, len(urlToFile))
			for k, v := range urlToFile {
				urlToFileCopy[k] = v
			}
			urlToFileMutex.Unlock()

			outputPath := filepath.Join(outputDir, data.filename)

			if data.isDocument {
				if err := os.WriteFile(outputPath, data.rawBytes, 0o600); err != nil {
					writeInfo(cmd.ErrOrStderr(), "  Error saving file: %v\n", err)
					continue
				}
			} else {
				markdown := converter.ConvertLinksToLocal(data.markdown, data.pageURL, urlToFileCopy)
				if err := os.WriteFile(outputPath, []byte(markdown), 0o600); err != nil {
					writeInfo(cmd.ErrOrStderr(), "  Error saving file: %v\n", err)
					continue
				}
			}

			writeInfo(cmd.OutOrStdout(), "  Saved: %s\n", outputPath)
			successCount++
		}

		writeInfo(cmd.OutOrStdout(), "\nSuccessfully saved %d pages to %s\n", successCount, outputDir)
	},
}

func init() {
	pf := crawldownCmd.PersistentFlags()
	pf.StringP("output", "o", "", "Output directory for saving Markdown files (enables crawl mode)")
	pf.StringP("output-file", "f", "", "Output file for single-page mode (default: stdout)")
	pf.IntP("depth", "d", 2, "Maximum crawl depth (crawl mode only)")
	pf.StringArrayP("exclude", "e", []string{}, "URL path prefixes to exclude from crawling (repeatable)")
	pf.StringArray("allowed-path", []string{}, "Only crawl URLs whose path starts with this prefix (repeatable)")
	pf.IntP("timeout", "t", 60, "Request timeout in seconds")
	pf.Int("delay", 0, "Delay between requests in seconds")
	pf.String("user-agent", "sdt/1.0", "HTTP user agent for requests")
	pf.Bool("ignore-robots-txt", false, "Ignore robots.txt restrictions")
	pf.Bool("follow-external", false, "Follow links to external domains")
	pf.Bool("download-docs", false, "Download linked documents such as PDF, Word, Office and text files")
	rootCmd.AddCommand(crawldownCmd)
}
