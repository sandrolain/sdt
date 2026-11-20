## sdt crawldown

Download a web page or site as Markdown

### Synopsis

Download web pages as Markdown and read them on stdout or keep them in files.

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
    written, and the exit code says what happened (see 'sdt --help'): 0 when
    every page was written, 3 when at least one was written and at least one
    failed, 1 when nothing was written at all.

Politeness: --delay <seconds> waits between URLs in a batch and between requests
while crawling; pass it when capturing many pages from one host.

Crawl bounds and document size:
  - --limit caps the pages a crawl fetches, sitemap seeds included; --depth caps
    how far it follows links.
  - --max-document-bytes refuses a document over the cap: nothing is written for
    it and the run counts a failure, so a truncated document is never produced.
  - A link skipped by a path filter, a document download that is off, single-page
    mode or the page cap is logged with its reason at debug. DEPTH_LIMIT and
    ROBOTS_TXT are enforced by the crawler engine and are not reported here.

```
sdt crawldown <url>... [flags]
```

### Options

```
      --allowed-path stringArray         Only crawl URLs whose path starts with this prefix (crawl mode only, repeatable)
      --allowed-path-regex stringArray   Only crawl URLs whose path matches this regex (crawl mode only, repeatable)
      --delay int                        Delay in seconds between captured URLs and between crawl requests
  -d, --depth int                        Maximum crawl depth (crawl mode only) (default 2)
      --download-docs[=true|false]       Also download linked documents such as PDF, Word, Office and text files (crawl and capture modes)
  -e, --exclude stringArray              URL path prefixes to exclude from crawling (crawl mode only, repeatable)
      --follow-external[=true|false]     Follow links to external domains (crawl mode only)
  -h, --help[=true|false]                help for crawldown
      --ignore-robots-txt[=true|false]   Ignore robots.txt restrictions (crawl mode only)
      --limit int                        Maximum number of pages to fetch in a crawl, sitemap seeds included (0 = unlimited; crawl mode only)
      --max-document-bytes int           Refuse a downloaded document larger than this many bytes (crawl and capture modes) (default 104857600)
      --no-frontmatter[=true|false]      Omit the YAML frontmatter block (stdout, file and capture modes)
      --no-title[=true|false]            Do not inject the page title as an H1 (stdout, file and capture modes)
  -o, --output string                    Output directory: crawl mode for one URL, capture mode for several
  -f, --output-file string               Write the single fetched page to this file (default: stdout)
      --sitemap[=true|false]             Seed the crawl from the site's sitemaps (crawl mode only)
  -t, --timeout int                      Request timeout in seconds (default 60)
      --user-agent string                HTTP user agent for requests (default "sdt/1.0")
```

### Options inherited from parent commands

```
      --file string             Input File
      --format string           Output format: text|json|yaml (default "text")
      --inb64 bytesBase64       Input Base 64
      --input string            Input String
      --log-format string       Log format: text|json (default "text")
      --no-color[=true|false]   Disable ANSI color codes
      --no-env[=true|false]     Do not load a project-root .env file
      --progress string         Progress narration on stderr: auto|human|json|none (default "auto")
      --quiet[=true|false]      Suppress informational messages, only output result
```

### SEE ALSO

* [sdt](sdt.md)	 - Smart Developer Tools
