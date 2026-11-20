## sdt env

Parse and manipulate .env files

### Options

```
  -h, --help[=true|false]   help for env
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
* [sdt env get](sdt_env_get.md)	 - Get a value from a .env file
* [sdt env merge](sdt_env_merge.md)	 - Merge multiple .env files (last value wins)
* [sdt env parse](sdt_env_parse.md)	 - Parse a .env file and output as JSON or shell exports
* [sdt env set](sdt_env_set.md)	 - Set or update a value in a .env file
