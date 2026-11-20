## sdt context wiki reference

Check the conditional reference→index bridge

### Synopsis

Check the reference bundles under context/refs/reference/.

A bundle is mandatory once a wiki page links at least one file in it: every
.md file in the bundle must then be cited by some wiki page. A bundle no page
links is optional and reports no finding; --strict turns an optional bundle
into a finding.

The check is read-only. Exit 1 on a mandatory gap (or any optional bundle with
--strict), 0 otherwise.

```
sdt context wiki reference [--strict] [flags]
```

### Options

```
  -h, --help[=true|false]     help for reference
      --strict[=true|false]   Treat an optional bundle with no citing page as a finding
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

* [sdt context wiki](sdt_context_wiki.md)	 - Knowledge graph operations (wiki lint)
