## sdt hmac

Compute HMAC of input using a secret key

### Synopsis

Compute HMAC (Hash-based Message Authentication Code) of input data.

Useful for verifying webhook signatures and message authenticity.

Algorithms: sha256 (default), sha512, sha384

Examples:
  echo -n "payload" | sdt hmac --key "secret"
  echo -n "payload" | sdt hmac --key "secret" --algo sha512
  echo -n "payload" | sdt hmac --key "secret" --format json
  sdt hmac --key "secret" --algo sha256 --file payload.bin

```
sdt hmac [flags]
```

### Options

```
      --algo string   Hash algorithm: sha256|sha512|sha384 (default "sha256")
  -h, --help          help for hmac
      --key string    Secret key for HMAC (required)
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

