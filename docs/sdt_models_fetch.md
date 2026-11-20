## sdt models fetch

Download and cache a static embedding model

### Synopsis

Download a static embedding model into the local cache
(os.UserCacheDir()/go-potion) so semantic search works offline afterwards.

Models (size grows with quality): BASE2M (~8 MB, 64 dims), BASE8M (~31 MB,
256 dims, default), MULTILINGUAL128M (~506 MB, 256 dims, 101 languages).

Examples:
  sdt models fetch
  sdt models fetch --model MULTILINGUAL128M
  sdt context search "hybrid search" --semantic

```
sdt models fetch [flags]
```

### Options

```
  -h, --help[=true|false]   help for fetch
      --model string        Model id: BASE2M|BASE8M|MULTILINGUAL128M (default BASE8M)
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

* [sdt models](sdt_models.md)	 - Manage local models for semantic search
