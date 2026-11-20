## sdt totp verify

Verify Code

### Synopsis

Verify Code

```
sdt totp verify [flags]
```

### Options

```
  -h, --help[=true|false]   help for verify
```

### Options inherited from parent commands

```
  -a, --account string          TOTP Account Name
  -l, --algorithm string        TOTP algorithm (SHA1, SHA256, SHA512, MD5) (default "SHA1")
  -c, --code string             TOTP Code
  -d, --digits int              TOTP digits (6, 8) (default 6)
      --file string             Input File
      --format string           Output format: text|json|yaml (default "text")
      --inb64 bytesBase64       Input Base 64
      --input string            Input String
  -r, --issuer string           TOTP Issuer
      --log-format string       Log format: text|json (default "text")
      --no-color[=true|false]   Disable ANSI color codes
      --no-env[=true|false]     Do not load a project-root .env file
  -p, --period uint             TOTP Period (default 30)
      --progress string         Progress narration on stderr: auto|human|json|none (default "auto")
      --quiet[=true|false]      Suppress informational messages, only output result
  -s, --secret string           TOTP Secret (Base 32)
```

### SEE ALSO

* [sdt totp](sdt_totp.md)	 - TOTP
