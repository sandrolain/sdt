# sdt

[![CI](https://github.com/sandrolain/sdt/workflows/CI/badge.svg)](https://github.com/sandrolain/sdt/actions/workflows/ci.yml)
[![Security](https://github.com/sandrolain/sdt/workflows/Security/badge.svg)](https://github.com/sandrolain/sdt/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sandrolain/sdt)](https://goreportcard.com/report/github.com/sandrolain/sdt)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Smart Developer Tools** - A collection of powerful CLI utilities for developers

> ⚠️ **Work in Progress**: Use at your own risk! This project is under active development.

## Features

- ✅ **Developer-Friendly** - Intuitive CLI interface with comprehensive help
- ✅ **Pipe Support** - Seamless integration with Unix pipes and standard I/O
- ✅ **Wide Range** - 50+ commands for encoding, hashing, JWT, time, and more
- ✅ **Web Interface** - WASM-based web application for browser usage
- ✅ **Cross-Platform** - Works on Linux, macOS, and Windows
- ✅ **Zero Dependencies** - Single binary with no runtime dependencies
- ✅ **Fast & Lightweight** - Built with Go for maximum performance

## Quick Start

### Installation

#### Using Go Install (Recommended)

```bash
go install github.com/sandrolain/sdt/cli@latest
```

#### Build from Source

```bash
# Clone the repository
git clone https://github.com/sandrolain/sdt
cd sdt

# Build and install using Task
task build

# Or build manually
go build -o bin/sdt ./cli
```

#### Pre-built Binaries

Download the latest release from the [releases page](https://github.com/sandrolain/sdt/releases).

### Quick Example

```bash
# Encode a string to base64
echo "Hello, World!" | sdt b64

# Generate a UUID
sdt uid v4

# Hash a string with SHA-256
echo "password" | sdt sha256

# Generate a JWT token
sdt jwt generate --secret mysecret --payload '{"user":"john"}'
```

## Usage

The CLI commands receive input data from:

- **Standard Input** (stdin) - via pipes or redirects
- **Command arguments** - without flags
- **File input** - using the `--input` flag

Output is sent to **Standard Output** (stdout), making it easy to chain commands.

If an error occurs, the command exits with code 1.

## Documentation

- **[Complete Command Reference](./docs/sdt.md)** - Auto-generated documentation for all commands
- **[Contributing](./CONTRIBUTING.md)** - Guidelines for contributing to the project

## Command Categories

### Encoding & Decoding

| Command | Description |
|---------|-------------|
| `b32` / `b32 dec` | Base32 encode/decode |
| `b64` / `b64 dec` | Base64 encode/decode |
| `b64url` / `b64url dec` | Base64 URL-safe encode/decode |
| `hex` / `hex dec` | Hexadecimal encode/decode |
| `url enc` / `url dec` | URL encode/decode |
| `html encode` / `html decode` | HTML entity encode/decode |

### Hashing & Cryptography

| Command | Description |
|---------|-------------|
| `md5` | MD5 hash |
| `sha1` | SHA-1 hash |
| `sha256` | SHA-256 hash |
| `sha384` | SHA-384 hash |
| `sha512` | SHA-512 hash |
| `bcrypt` / `bcrypt verify` | Bcrypt hash and verification |

### JWT & Tokens

| Command | Description |
|---------|-------------|
| `jwt parse` | Parse JWT token |
| `jwt claims` | Extract JWT claims |
| `jwt valid` | Validate JWT token |

### UUID & IDs

| Command | Description |
|---------|-------------|
| `uid v4` | Generate UUID v4 |
| `uid nano` | Generate Nano ID |
| `uid ks` | Generate KSUID |

### Time & Date

| Command | Description |
|---------|-------------|
| `time unix` | Unix timestamp |
| `time iso` | ISO 8601 timestamp |
| `time http` | HTTP date format |

### String Operations

| Command | Description |
|---------|-------------|
| `string uppercase` | Convert to UPPERCASE |
| `string lowercase` | Convert to lowercase |
| `string titlecase` | Convert to Title Case |
| `string count` | Count characters/words/lines |
| `string escape` | Escape special characters |

### JSON Operations

| Command | Description |
|---------|-------------|
| `json pretty` | Pretty-print JSON |
| `json minify` | Minify JSON |
| `json valid` | Validate JSON |

### Compression

| Command | Description |
|---------|-------------|
| `gzip` | Compress with gzip |
| `gunzip` | Decompress gzip |

### Network & Utilities

| Command | Description |
|---------|-------------|
| `http` | Make HTTP requests |
| `ipinfo` | Get IP information |
| `nslookup` | DNS lookup |
| `qrcode` | Generate QR code |
| `totp` | TOTP operations |

See the [complete documentation](./docs/sdt.md) for all commands and options.

## Common Use Cases

### Working with JWT Tokens

```bash
# Parse a JWT token
echo "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." | sdt jwt parse

# Validate a JWT token
echo "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." | sdt jwt valid --secret mysecret

# Extract claims from JWT
echo "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." | sdt jwt claims
```

### Data Encoding Chains

```bash
# Encode to base64, then URL encode
echo "Hello, World!" | sdt b64 | sdt url enc

# Hash with SHA-256 and encode as hex
echo "password" | sdt sha256 | sdt hex
```

### JSON Processing

```bash
# Validate and pretty-print JSON
cat data.json | sdt json valid && cat data.json | sdt json pretty

# Minify JSON for transmission
cat large-config.json | sdt json minify > config.min.json
```

### Password & Security

```bash
# Generate a secure password
sdt password --length 32 --symbols

# Hash a password with bcrypt
echo "mypassword" | sdt bcrypt

# Verify bcrypt password
echo "mypassword" | sdt bcrypt verify '$2a$10$...'
```

### TOTP Authentication

```bash
# Generate TOTP URI for QR code
sdt totp uri --issuer MyApp --account user@example.com

# Generate TOTP code
sdt totp code --secret JBSWY3DPEHPK3PXP

# Verify TOTP code
sdt totp verify --secret JBSWY3DPEHPK3PXP --code 123456
```

## Web Interface

SDT includes a WASM-based web interface that runs entirely in your browser. No server needed!

### Running the Web Interface Locally

```bash
# Build the web interface
task build:web

# Serve it locally
task serve:web
# Or use Docker
docker run -p 3000:3000 sandrolain/sdt:latest
```

The web interface provides:

- All CLI commands in a user-friendly interface
- File upload support
- Clipboard integration
- No data sent to any server (100% client-side)

## Development

### Prerequisites

- Go 1.22 or later
- [Task](https://taskfile.dev/) (recommended) or standard Go tools
- Docker (optional, for containerization)

### Building

```bash
# Build the CLI binary
task build

# Build with compression
task build:compress

# Build WASM version
task wasm:build

# Build web interface
task build:web
```

### Testing

```bash
# Run all tests
task test

# Run tests with coverage
task test:coverage

# Run benchmarks
task test:bench

# Run all checks (fmt, lint, vet, test, security)
task check
```

### Code Quality

```bash
# Format code
task fmt

# Run linter
task lint

# Run go vet
task vet

# Run security scan
task gosec

# Run vulnerability scan
task trivy
```

## Docker

### Using Pre-built Image

```bash
# Pull the image
docker pull sandrolain/sdt:latest

# Run the web interface
docker run -p 3000:3000 sandrolain/sdt:latest
```

### Building Docker Image

```bash
# Build for current platform
task docker:build

# Build multi-arch image
task docker:build:multi
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes with tests
4. Run quality checks (`task check`)
5. Commit your changes following [Conventional Commits](https://www.conventionalcommits.org/)
6. Push to your branch
7. Open a Pull Request

See [CONTRIBUTING.md](./CONTRIBUTING.md) for detailed guidelines.

## Roadmap

See the [Wishlist](#wishlist) section below for planned features and improvements.

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
  - [x] Commands without `sdt`
  - [x] Button for quick help
  - [ ] Commands selection
    - [ ] Various types of conversion
    - [x] Bytes b64
    - [x] Bytes hex
  - [x] Input from textarea
  - [x] Input from file
  - [x] Flag for inputs as B64
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
- [x] My IP (<https://ipapi.co/>)
- [x] IP Lookup (<https://ipapi.co/>)
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

## External Libraries Used

- [cobra](https://github.com/spf13/cobra) - CLI framework
- [clipboard](https://github.com/atotto/clipboard) - Clipboard access
- [jwt](https://github.com/golang-jwt/jwt) - JWT handling
- [colorjson](https://github.com/TylerBrock/colorjson) - JSON colorization
- [golorem](https://github.com/drhodes/golorem) - Lorem ipsum generator
- [beeep](https://github.com/gen2brain/beeep) - Desktop notifications
- [go-password](https://github.com/sethvargo/go-password) - Password generation
- [go-qrcode](https://github.com/skip2/go-qrcode) - QR code generation
- [otp](https://github.com/pquerna/otp) - TOTP/HOTP
- [uuid](https://github.com/google/uuid) - UUID generation
- [go-nanoid](https://github.com/matoous/go-nanoid) - Nano ID generation
- [ksuid](https://github.com/segmentio/ksuid) - K-Sortable Unique Identifiers
- [go-version](https://github.com/christopherhein/go-version) - Version management

## License

[MIT License](LICENSE)

Copyright (c) 2025 Sandro Lain

## Support

- 📖 [Documentation](./docs/sdt.md)
- 💬 [Issues](https://github.com/sandrolain/sdt/issues)
- 🌟 [GitHub](https://github.com/sandrolain/sdt)

---

Made with ❤️ by [Sandro Lain](https://github.com/sandrolain)
