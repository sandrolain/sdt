## sdt context wiki comprehension

Select the two-page-per-book comprehension sample

### Synopsis

Select a deterministic six-page sample: two concept pages per book, one
with the most claims (dense) and one with the fewest (boundary). The command
never records a verdict; the receipt records pass/partial/fail with the ids.

With --book (repeatable) only those index pages are sampled; without it every
entity page carrying a refs/ source is used.

```
sdt context wiki comprehension [--book <index>] [flags]
```

### Options

```
      --book stringArray    Index page to sample (repeatable; default: every entity page with a refs/ source)
  -h, --help[=true|false]   help for comprehension
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
