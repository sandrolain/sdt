# sdt

**Smart Developer Tools**

Collection of CLI utilities for developers

**W.I.P.**: *Use at your own risk!*

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

### DevOps Wishlist

- [ ] Unit Tests!!!
- [x] Use [UPX](https://upx.github.io/)

### Features Wishlist

- [ ] Color conversion
- [ ] [JQ integration](https://github.com/itchyny/gojq)
- [ ] Template with [mustache](https://github.com/cbroglie/mustache)
- [ ] Data faker
- [x] File change watcher with command execution
- [ ] Interval watcher with command execution
- [x] RegExp all matches
- [x] RegExp replace
- [x] Data conversion between:
  - [x] [Query string](https://github.com/hetiansu5/urlquery)
  - [x] JSON
  - [x] [YAML](https://github.com/go-yaml/yaml)
  - [x] [TOML](https://github.com/pelletier/go-toml)
  - [x] CSV
- [x] Save []string to multiple files
- [x] Static file server
- [ ] WASM web app:
  - [ ] Commands without `sdt`
  - [ ] Button for quick help
  - [ ] Commands selection
    - [ ] Various types of conversion
    - [x] Bytes b64
    - [x] Bytes hex
  - [x] Input from textarea
  - [ ] Input from file
  - [ ] Flag for inputs as B64
  - [x] Output to textarea
  - [ ] Output to file
- [ ] GUI app:
  - [ ] Commands selection
  - [ ] Input from textarea
  - [ ] Input from file
  - [ ] Output to textarea
  - [ ] Output to file
- [ ] Docker images
  - [x] WASM webapp
  - [ ] CLI as Docker image ???
- [ ] HTTP service
  - [ ] commands as http APIs
- [x] Edit config file
- [x] Request input from CLI
- [x] Global --input (file) flag
- [x] Base 32 encode/decode
- [x] HTTP request
- [x] RegEx match
- [x] Lorem Ipsum
- [x] QR code generation
- [x] TOTP
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
- [x] Base 64 encode/decode
- [x] URL encode / decode
- [x] Time unix
- [x] Time ISO
- [x] Time UTC
- [x] JWT validate
- [x] JWT claims
- [x] Gzip / Gunzip
- [x] My IP (https://ipapi.co/)
- [x] IP Lookup (https://ipapi.co/)
- [x] NS lookup
- [ ] QR code reader
- [ ] Base 36
- [ ] HMAC generator
- [ ] Validator
  - [ ] JSON
  - [ ] YAML
  - [ ] TOML
  - [ ] CSV
- [ ] String
  - [x] Character count
  - [x] Word count
  - [x] Line count
  - [x] UPPER CASE
  - [x] lower case
  - [ ] camelCase
  - [x] Capital Case
  - [x] CONSTANT_CASE (Upper case + Replace spaces with character)
  - [x] dot.case (Replace spaces with character)
  - [x] Header-Case (Capital case + Replace spaces with character)
  - [x] param-case (Replace spaces with character)
  - [x] snake_case (Replace spaces with character)
  - [ ] Slug generator
  - [x] html encode, html decode
  - [ ] Sort Lines (reverse, shuffle) (by content, by length)


## External libraries used
- [cobra](https://github.com/spf13/cobra)
- [clipboard](https://github.com/atotto/clipboard)
- [jwt](https://github.com/golang-jwt/jwt)
- [colorjson](https://github.com/TylerBrock/colorjson)
- [golorem](https://github.com/drhodes/golorem)
- [beeep](https://github.com/gen2brain/beeep)
- [go-password](https://github.com/sethvargo/go-password)
- [go-qrcode](https://github.com/skip2/go-qrcode)
- [otp](https://github.com/pquerna/otp)
- [uuid](https://github.com/google/uuid)
- [go-nanoid](https://github.com/matoous/go-nanoid)
- [ksuid](https://github.com/segmentio/ksuid) 
- [go-version](https://github.com/christopherhein/go-version)

