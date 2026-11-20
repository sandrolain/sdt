## sdt q

Query and patch JSON files by gjson path

### Synopsis

Query and patch JSON files with a gjson path: a dotted path (a.b),
array indexes (a.0), wildcards (*, ?), queries (#(...)), projections (a.#.b)
and modifiers (@reverse, @pretty, @keys, @flatten).

JSON only: a .yaml/.yml argument is rejected with a named error. Convert YAML
first with "sdt conv --in yaml --out json" (usable on stdin).

Exit codes: 0 success · 1 path/file not found · 2 malformed JSON or an
unsupported input format. Diagnostics go to stderr, never to stdout.

### Options

```
  -h, --help[=true|false]   help for q
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
* [sdt q del](sdt_q_del.md)	 - Delete the value at a gjson path, preserving the rest of the file
* [sdt q get](sdt_q_get.md)	 - Print the value at a gjson path
* [sdt q set](sdt_q_set.md)	 - Set the value at a gjson path, preserving the rest of the file
