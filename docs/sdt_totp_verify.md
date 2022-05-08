## sdt totp verify

Verify Code

### Synopsis

Verify Code

```
sdt totp verify [flags]
```

### Options

```
  -h, --help   help for verify
```

### Options inherited from parent commands

```
  -a, --account string     TOTP Account Name
  -l, --algorithm string   TOTP algorithm (SHA1, SHA256, SHA512, MD5) (default "SHA1")
  -c, --code string        TOTP Code
  -d, --digits int         TOTP digits (6, 8) (default 6)
  -f, --file string        Input File
  -i, --input              Input Prompt
  -r, --issuer string      TOTP Issuer
  -p, --period uint        TOTP Period (default 30)
  -s, --secret string      TOTP Secret (Base 32)
```

### SEE ALSO

* [sdt totp](sdt_totp.md)	 - TOTP

