## sdt crawldown

Download a web page or site as Markdown

### Synopsis

Download a web page or entire website and convert the content to Markdown.

Without --output, fetches the given URL as a single page and prints the Markdown to stdout
(or to a file when --output-file is specified).
With --output <dir>, crawls the full site starting from the given URL and saves each page
as a separate .md file in the output directory.

```
sdt crawldown <url> [flags]
```

### Options

```
      --allowed-path stringArray   Only crawl URLs whose path starts with this prefix (repeatable)
      --delay int                  Delay between requests in seconds
  -d, --depth int                  Maximum crawl depth (crawl mode only) (default 2)
      --download-docs              Download linked documents such as PDF, Word, Office and text files
  -e, --exclude stringArray        URL path prefixes to exclude from crawling (repeatable)
      --follow-external            Follow links to external domains
  -h, --help                       help for crawldown
      --ignore-robots-txt          Ignore robots.txt restrictions
  -o, --output string              Output directory for saving Markdown files (enables crawl mode)
  -f, --output-file string         Output file for single-page mode (default: stdout)
  -t, --timeout int                Request timeout in seconds (default 60)
      --user-agent string          HTTP user agent for requests (default "sdt/1.0")
```

### Options inherited from parent commands

```
      --file string         Input File
      --format string       Output format: text|json|yaml (default "text")
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
      --no-color            Disable ANSI color codes
      --quiet               Suppress informational messages, only output result
```

### SEE ALSO

* [sdt](sdt.md)	 - Smart Developer Tools

