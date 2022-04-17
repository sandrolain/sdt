# sdt

**Smart Developer Tools**

Collection of CLI utilities for developers

The CLI commands receive input data (where provided) from the Standard Input or from an additional parameter without a flag, and output data to the Standard Output.  
If an error occurs, the command will be terminated with exit code 1.

## Installation with local build

The installation script build the binary directory to your GO /bin directory.

Yout need GO installed on your machine, and assure that the GOPATH/bin directory is exported in your PATH.

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

so...

1. Clone this repository locally
   ```sh
   git clone https://github.com/sandrolain/sdt
   ```
2. Run the script `install.sh`;
   ```sh
   cd ./sdt
   sh ./install.sh
   ```



## Documentation

Auto-generated CLI documentation is available [here](./docs/sdt.md).

## Wishlist

- [ ] Request input from CLI
- [ ] Color conversion
- [x] RegEx match
- [ ] RegExp all matches
- [ ] RegExp replace
- [ ] Query string to JSON and reverse
- [ ] Lorem Ipsum
- [x] QR code generation
- [ ] HTTP request
- [ ] JSON to YAML and reverse
- [ ] Global --inputfile parameter
- [ ] Templating with mustache: https://github.com/cbroglie/mustache
- [ ] WASM
- [x] CSV to JSON
- [x] Backslash escape/unsescape
- [x] String case converter
- [x] JSON validator
- [x] Read / Write File
- [x] JSON minify
- [x] JSON prettify
- [x] Bcrypt hash
- [x] Bcrypt check
- [x] Hash SHA-384
- [x] Hash SHA-256
- [x] Hash sha1
- [x] Random Bytes
- [x] UUID v4
- [x] Base64 encode/decode
- [x] URL encode / decode
- [x] Time unix
- [x] Time ISO
- [x] Time UTC
- [x] JWT validate
- [x] JWT claims
- [x] Gzip / Gunzip

