package converter

import (
	"fmt"
	"mime"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
)

// Options defines converter configuration.
type Options struct {
	Domain string
}

// markdownExt is the markdown file extension used when building filenames.
const markdownExt = ".md"

// Converter handles HTML to Markdown conversion.
type Converter struct {
	converter *md.Converter
	options   Options
}

// NewConverter creates a new Converter instance.
func NewConverter(opts Options) (*Converter, error) {
	conv := md.NewConverter(opts.Domain, true, nil)

	conv.Use(plugin.GitHubFlavored())
	conv.Use(plugin.Table())
	conv.Use(plugin.TaskListItems())
	conv.Use(plugin.Strikethrough("~~"))

	return &Converter{
		converter: conv,
		options:   opts,
	}, nil
}

// Convert converts an HTML string to Markdown.
func (c *Converter) Convert(html string) (string, error) {
	if html == "" {
		return "", fmt.Errorf("empty HTML content")
	}

	markdown, err := c.converter.ConvertString(html)
	if err != nil {
		return "", fmt.Errorf("conversion failed: %w", err)
	}

	return cleanup(markdown), nil
}

// cleanup runs the deterministic post-conversion passes in order. Each pass is
// independent and unit-tested; the order matters only where a later pass assumes
// an earlier one's output.
func cleanup(markdown string) string {
	markdown = removeSkipLinks(markdown)
	markdown = repairMultilineLinks(markdown)
	markdown = scrubBase64Images(markdown)
	return collapseBlankLines(markdown)
}

// skipLinkTextRe matches the text of a "skip to content"-style link.
var skipLinkTextRe = regexp.MustCompile(`(?i)^\s*(skip to (main )?(content|navigation)|skip navigation|jump to (main )?content)\s*$`)

// removeSkipLinks drops inline links whose text is a skip-to-content phrase,
// leaving the surrounding text.
func removeSkipLinks(markdown string) string {
	re := regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	return re.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) == 3 && skipLinkTextRe.MatchString(parts[1]) {
			return ""
		}
		return match
	})
}

// repairMultilineLinks joins a link destination broken across lines (`](` then
// the URL on its own line) into one line.
func repairMultilineLinks(markdown string) string {
	re := regexp.MustCompile(`\]\(([^)]*)\)`)
	return re.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) != 2 || !strings.Contains(parts[1], "\n") {
			return match
		}
		return "](" + strings.Join(strings.Fields(parts[1]), " ") + ")"
	})
}

// base64ImageRe matches a markdown image whose destination is an inline
// `data:` URI.
var base64ImageRe = regexp.MustCompile(`!\[([^\]]*)\]\(\s*data:[^)]*\)`)

// scrubBase64Images replaces a base64 `data:` image with its alt text (or
// removes it), so a captured page does not carry megabytes of inline pixels.
func scrubBase64Images(markdown string) string {
	return base64ImageRe.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := base64ImageRe.FindStringSubmatch(match)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
		return ""
	})
}

// collapseBlankLines collapses runs of 3+ newlines and trims the result.
func collapseBlankLines(markdown string) string {
	re := regexp.MustCompile(`\n{3,}`)
	markdown = re.ReplaceAllString(markdown, "\n\n")
	return strings.TrimSpace(markdown)
}

// ConvertLinksToLocal rewrites absolute URLs in markdown to local .md file references
// using the provided URL-to-filename mapping.
func ConvertLinksToLocal(markdown string, baseURL string, urlToFileMap map[string]string) string {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return markdown
	}

	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	return re.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		linkText := parts[1]
		linkURL := parts[2]

		if strings.HasPrefix(linkURL, "#") ||
			strings.HasPrefix(linkURL, "mailto:") ||
			strings.HasPrefix(linkURL, "javascript:") {
			return match
		}

		parsedLink, err := url.Parse(linkURL)
		if err != nil {
			return match
		}

		if !parsedLink.IsAbs() {
			parsedLink = parsedBase.ResolveReference(parsedLink)
		}

		fullURL := parsedLink.Scheme + "://" + parsedLink.Host + strings.TrimSuffix(parsedLink.Path, "/")
		if parsedLink.RawQuery != "" {
			fullURL += "?" + parsedLink.RawQuery
		}

		if localFile, exists := urlToFileMap[fullURL]; exists {
			if parsedLink.Fragment != "" {
				return fmt.Sprintf("[%s](%s#%s)", linkText, localFile, parsedLink.Fragment)
			}

			return fmt.Sprintf("[%s](%s)", linkText, localFile)
		}

		cleanURL := parsedLink.Scheme + "://" + parsedLink.Host + strings.TrimSuffix(parsedLink.Path, "/")
		if localFile, exists := urlToFileMap[cleanURL]; exists {
			if parsedLink.Fragment != "" {
				return fmt.Sprintf("[%s](%s#%s)", linkText, localFile, parsedLink.Fragment)
			}

			return fmt.Sprintf("[%s](%s)", linkText, localFile)
		}

		return match
	})
}

// GenerateFilename creates a safe .md filename from a URL.
func GenerateFilename(pageURL string) string {
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return "index" + markdownExt
	}

	path := parsedURL.Path
	query := parsedURL.RawQuery

	if path == "" || path == "/" {
		if query != "" {
			filename := "index-" + sanitizeFilename(query)
			if !strings.HasSuffix(filename, markdownExt) {
				filename += markdownExt
			}

			return filename
		}

		return "index" + markdownExt
	}

	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	filename := strings.ReplaceAll(path, "/", "-")

	if query != "" {
		filename = filename + "-" + query
	}

	filename = sanitizeFilename(filename)

	if !strings.HasSuffix(filename, markdownExt) {
		if filepath.Ext(filename) != "" {
			filename = strings.TrimSuffix(filename, filepath.Ext(filename))
		}

		filename += markdownExt
	}

	return filename
}

// FilenameFromDisposition resolves the filename from a Content-Disposition
// header, preferring the RFC 5987 `filename*` form then `filename=`, sanitized;
// "" when the header carries no usable filename. mime.ParseMediaType decodes the
// RFC 5987 form into the `filename` parameter.
func FilenameFromDisposition(header string) string {
	if header == "" {
		return ""
	}
	if _, params, err := mime.ParseMediaType(header); err == nil {
		if name := strings.TrimSpace(params["filename"]); name != "" {
			return sanitizeFilename(filepath.Base(name))
		}
	}
	return ""
}

// GenerateAssetFilename creates a safe file name for downloaded non-HTML resources.
func GenerateAssetFilename(pageURL string) string {
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return "download.bin"
	}

	path := parsedURL.Path
	query := parsedURL.RawQuery
	docExt := DocumentExtension(path)

	if path == "" || path == "/" {
		filename := "index"
		if query != "" {
			filename += "-" + sanitizeFilename(query)
		}
		if docExt != "" {
			filename += docExt
		} else {
			filename += ".bin"
		}
		return filename
	}

	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	filename := strings.ReplaceAll(path, "/", "-")
	filename = sanitizeFilename(filename)

	ext := filepath.Ext(filename)
	if docExt != "" && ext != docExt {
		if ext != "" {
			filename = strings.TrimSuffix(filename, ext)
		}
		ext = docExt
	}

	if ext == "" {
		if query != "" {
			filename = filename + "-" + sanitizeFilename(query)
		}
		if ext == "" {
			ext = ".bin"
		}
		filename += ext
		return filename
	}

	if query != "" {
		filename = strings.TrimSuffix(filename, ext) + "-" + sanitizeFilename(query) + ext
		return filename
	}

	filename += ext
	return filename
}

// DocumentExtensions is the single registry of convertible document
// extensions, shared by the filename builder and the crawler's link filter.
var DocumentExtensions = []string{
	".pdf",
	".doc",
	".docx",
	".xls",
	".xlsx",
	".ppt",
	".pptx",
	".odt",
	".ods",
	".odp",
	".rtf",
	".txt",
	".epub",
	markdownExt,
	".markdown",
	".csv",
}

// DocumentExtension returns the registered extension matching path (a file
// path or a URL path), or "" when path is not a convertible document.
func DocumentExtension(path string) string {
	lowerPath := strings.ToLower(path)
	for _, ext := range DocumentExtensions {
		if strings.HasSuffix(lowerPath, ext) || strings.Contains(lowerPath, ext+"/") || strings.Contains(lowerPath, ext+"?") || strings.Contains(lowerPath, ext+"#") {
			return ext
		}
	}

	return ""
}

func sanitizeFilename(filename string) string {
	re := regexp.MustCompile(`[<>:"/\\|?*=&]`)
	filename = re.ReplaceAllString(filename, "-")

	re = regexp.MustCompile(`-+`)
	filename = re.ReplaceAllString(filename, "-")

	filename = strings.Trim(filename, "-")

	if len(filename) > 200 {
		filename = filename[:200]
	}

	return filename
}
