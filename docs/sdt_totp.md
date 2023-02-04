## sdt totp

TOTP

### Synopsis

Time-based One Time Password

### Options

```
  -a, --account string     TOTP Account Name
  -l, --algorithm string   TOTP algorithm (SHA1, SHA256, SHA512, MD5) (default "SHA1")
  -c, --code string        TOTP Code
  -d, --digits int         TOTP digits (6, 8) (default 6)
  -h, --help               help for totp
  -r, --issuer string      TOTP Issuer
  -p, --period uint        TOTP Period (default 30)
  -s, --secret string      TOTP Secret (Base 32)
```

### Options inherited from parent commands

```
      --file string         Input File
      --inb64 bytesBase64   Input Base 64
      --input string        Input String
```

### SEE ALSO

* [sdt](sdt.md)	 - Smart Developer Tools
* [sdt totp code](sdt_totp_code.md)	 - Generate Code
* [sdt totp image](sdt_totp_image.md)	 - Generate QR code Image
* [sdt totp uri](sdt_totp_uri.md)	 - Generate URI
* [sdt totp verify](sdt_totp_verify.md)	 - Verify Code

