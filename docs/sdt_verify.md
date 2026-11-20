## sdt verify

Verify a signature against data using a public key

### Synopsis

Verify that the input data matches the given base64-encoded signature
using the provided PEM public key.

Examples:
  echo -n "payload" | sdt verify --key public.pem --sig <base64>
  sdt verify --key public.pem --sig <base64> --algo rsa-sha512 --file payload.bin

```
sdt verify [flags]
```

### Options

```
      --algo string         Signing algorithm: rsa-sha256|rsa-sha512|ecdsa-sha256|ecdsa-sha512|ed25519 (default "rsa-sha256")
  -h, --help[=true|false]   help for verify
      --key string          Path to PEM public key file (required)
      --sig string          Base64-encoded signature to verify (required)
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
