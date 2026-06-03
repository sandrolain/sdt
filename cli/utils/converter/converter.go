package converter

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
)

// Options defines converter configuration.
type Options struct {
	Domain           string
	BulletListMarker string
	CodeBlockStyle   string
	EmDelimiter      string
	StrongDelimiter  string
	LinkStyle        string
}

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

	return cleanMarkdown(markdown), nil
}

func cleanMarkdown(markdown string) string {
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
		return "index.md"
	}

	path := parsedURL.Path
	query := parsedURL.RawQuery

	if path == "" || path == "/" {
		if query != "" {
			filename := "index-" + sanitizeFilename(query)
			if !strings.HasSuffix(filename, ".md") {
				filename += ".md"
			}

			return filename
		}

		return "index.md"
	}

	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	filename := strings.ReplaceAll(path, "/", "-")

	if query != "" {
		filename = filename + "-" + query
	}

	filename = sanitizeFilename(filename)

	if !strings.HasSuffix(filename, ".md") {
		if filepath.Ext(filename) != "" {
			filename = strings.TrimSuffix(filename, filepath.Ext(filename))
		}

		filename += ".md"
	}

	return filename
}

// GenerateAssetFilename creates a safe file name for downloaded non-HTML resources.
func GenerateAssetFilename(pageURL string) string {
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return "download.bin"
	}

	path := parsedURL.Path
	query := parsedURL.RawQuery
	docExt := findDocumentExtension(path)

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

func findDocumentExtension(path string) string {
	extensions := []string{
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
		".md",
		".markdown",
		".csv",
	}

	lowerPath := strings.ToLower(path)
	for _, ext := range extensions {
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
