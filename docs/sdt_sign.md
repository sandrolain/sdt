## sdt sign

Sign data with a private key

### Synopsis

Sign input data using a PEM private key. Outputs a base64-encoded signature.

Supported algorithms:
  rsa-sha256    (default for RSA keys)
  rsa-sha512
  ecdsa-sha256  (default for ECDSA keys)
  ecdsa-sha512
  ed25519

Examples:
  echo -n "payload" | sdt sign --key private.pem
  echo -n "payload" | sdt sign --key private.pem --algo rsa-sha512
  sdt sign --key ed25519.pem --algo ed25519 --file payload.bin

```
sdt sign [flags]
```

### Options

```
      --algo string   Signing algorithm: rsa-sha256|rsa-sha512|ecdsa-sha256|ecdsa-sha512|ed25519 (default "rsa-sha256")
  -h, --help          help for sign
      --key string    Path to PEM private key file (required)
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

