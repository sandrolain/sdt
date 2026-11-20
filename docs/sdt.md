## sdt

Smart Developer Tools

### Synopsis


                             dddddddd
                             d::::::d        tttt
                             d::::::d     ttt:::t
                             d::::::d     t:::::t
                             d:::::d      t:::::t
    ssssssssss       ddddddddd:::::dttttttt:::::ttttttt
  ss::::::::::s    dd::::::::::::::dt:::::::::::::::::t
ss:::::::::::::s  d::::::::::::::::dt:::::::::::::::::t
s::::::ssss:::::sd:::::::ddddd:::::dtttttt:::::::tttttt
 s:::::s  ssssss d::::::d    d:::::d      t:::::t
   s::::::s      d:::::d     d:::::d      t:::::t
      s::::::s   d:::::d     d:::::d      t:::::t
ssssss   s:::::s d:::::d     d:::::d      t:::::t    tttttt
s:::::ssss::::::sd::::::ddddd::::::dd     t::::::tttt:::::t
s::::::::::::::s  d:::::::::::::::::d     tt::::::::::::::t
 s:::::::::::ss    d:::::::::ddd::::d       tt:::::::::::tt
  sssssssssss       ddddddddd   ddddd         ttttttttttt

Smart Developer Tools is a collection of CLI utilities for developers

Exit codes:
  0  the command did its work
  1  the command failed; nothing usable was produced (a usage error reports 1 too,
     so a script keyed on 1 keeps working)
  2  a command with its own verdict scale reports 2 for its error case: the wiki
     lint and the wiki commands already do, and their warnings stay 1
  3  partial success: some items worked, some did not, and the result is usable
     (crawldown with at least one page written and at least one failed)
  4  a decision or a verdict is required from the caller, not a repair of the
     command: it prints what it needs (context close refused for unfinished work
     prints the open items and the two commands that resolve them; learn answer
     reports "cannot grade")

### Options

```
      --file string             Input File
      --format string           Output format: text|json|yaml (default "text")
  -h, --help[=true|false]       help for sdt
      --inb64 bytesBase64       Input Base 64
      --input string            Input String
      --log-format string       Log format: text|json (default "text")
      --no-color[=true|false]   Disable ANSI color codes
      --no-env[=true|false]     Do not load a project-root .env file
      --progress string         Progress narration on stderr: auto|human|json|none (default "auto")
      --quiet[=true|false]      Suppress informational messages, only output result
```

### SEE ALSO

* [sdt agent](sdt_agent.md)	 - Agent instruction tools (AGENTS.md, instruction files)
* [sdt b32](sdt_b32.md)	 - B32 Encode
* [sdt b64](sdt_b64.md)	 - B64 Encode
* [sdt b64url](sdt_b64url.md)	 - B64 URL Encode
* [sdt bcrypt](sdt_bcrypt.md)	 - Bcrypt password
* [sdt bytes](sdt_bytes.md)	 - Random Bytes
* [sdt cert](sdt_cert.md)	 - Inspect TLS/X.509 certificates
* [sdt chunk](sdt_chunk.md)	 - Transient chunk search over an artifact (no index written)
* [sdt color](sdt_color.md)	 - Colour tools
* [sdt completion](sdt_completion.md)	 - Generate the autocompletion script for the specified shell
* [sdt config](sdt_config.md)	 - Configuration Tools
* [sdt context](sdt_context.md)	 - Context Tools (context/ work files)
* [sdt conv](sdt_conv.md)	 - Data Conversion
* [sdt crawldown](sdt_crawldown.md)	 - Download a web page or site as Markdown
* [sdt diff](sdt_diff.md)	 - Compare two files and output differences
* [sdt dns](sdt_dns.md)	 - DNS lookup for a host
* [sdt doc2md](sdt_doc2md.md)	 - Convert documents to Markdown with the first runnable converter
* [sdt docs](sdt_docs.md)	 - Generate CLI docs
* [sdt edit](sdt_edit.md)	 - Replace one literal occurrence in a file, guarded and atomic
* [sdt env](sdt_env.md)	 - Parse and manipulate .env files
* [sdt extract](sdt_extract.md)	 - Extract structured items from text input
* [sdt figure](sdt_figure.md)	 - Extract a book's figures with their captions into assets
* [sdt find](sdt_find.md)	 - Find Files By Path
* [sdt gain](sdt_gain.md)	 - Report what SDT actually did, counted from the local usage log
* [sdt grep](sdt_grep.md)	 - Grep File Content
* [sdt gunzip](sdt_gunzip.md)	 - Gunzip
* [sdt gzip](sdt_gzip.md)	 - Gzip
* [sdt hex](sdt_hex.md)	 - Hex Encoding
* [sdt hmac](sdt_hmac.md)	 - Compute HMAC of input using a secret key
* [sdt html](sdt_html.md)	 - HTML Tools
* [sdt http](sdt_http.md)	 - HTTP client
* [sdt image](sdt_image.md)	 - Image tools
* [sdt ipinfo](sdt_ipinfo.md)	 - Classify an IP locally or look up an explicit target
* [sdt json](sdt_json.md)	 - JSON Tools
* [sdt jwt](sdt_jwt.md)	 - JWT Tools
* [sdt keypair](sdt_keypair.md)	 - Key pair PEMs
* [sdt learn](sdt_learn.md)	 - Learn, review and record concepts against the local wiki
* [sdt ls](sdt_ls.md)	 - List a directory, one or more levels deep
* [sdt manifest](sdt_manifest.md)	 - Emit a machine-readable manifest of all available commands
* [sdt material](sdt_material.md)	 - Export a study guide without its authoring machinery
* [sdt md5](sdt_md5.md)	 - MD5
* [sdt models](sdt_models.md)	 - Manage local models for semantic search
* [sdt password](sdt_password.md)	 - Generate password
* [sdt port](sdt_port.md)	 - Check if a TCP port is open on a host
* [sdt q](sdt_q.md)	 - Query and patch JSON files by gjson path
* [sdt qrcode](sdt_qrcode.md)	 - QR code
* [sdt read](sdt_read.md)	 - Read a file or a line range of it
* [sdt regexp](sdt_regexp.md)	 - RegExp matching
* [sdt research](sdt_research.md)	 - Auditable research runs: manifest, provenance and resume
* [sdt schema](sdt_schema.md)	 - Generate JSON Schema for SDT commands
* [sdt sha1](sdt_sha1.md)	 - SHA-1
* [sdt sha256](sdt_sha256.md)	 - SHA-256
* [sdt sha384](sdt_sha384.md)	 - SHA-384
* [sdt sha512](sdt_sha512.md)	 - SHA-512
* [sdt sign](sdt_sign.md)	 - Sign data with a private key
* [sdt string](sdt_string.md)	 - String Tools
* [sdt template](sdt_template.md)	 - Render a Go text/template with JSON or YAML data
* [sdt time](sdt_time.md)	 - Time Tools
* [sdt tokens](sdt_tokens.md)	 - Count approximate LLM tokens for the given text
* [sdt totp](sdt_totp.md)	 - TOTP
* [sdt truncate](sdt_truncate.md)	 - Truncate text using an approximate token limit
* [sdt uid](sdt_uid.md)	 - Unique ID
* [sdt url](sdt_url.md)	 - URL Tools
* [sdt verify](sdt_verify.md)	 - Verify a signature against data using a public key
* [sdt version](sdt_version.md)	 - Version will output the current build information
* [sdt vman](sdt_vman.md)	 - Version Manager
* [sdt write](sdt_write.md)	 - Write bytes to a file
