# Docker Setup for SDT

This directory contains Docker configuration for the SDT web interface.

## Quick Start

### Using Docker Run

```bash
docker run -p 3000:3000 sandrolain/sdt:latest
```

Then open <http://localhost:3000> in your browser.

### Using Docker Compose

```bash
# From the docker directory
docker-compose up -d

# Or from the project root
docker-compose -f docker/docker-compose.yml up -d
```

### Custom Port

```bash
# Run on port 8080
docker run -p 8080:3000 sandrolain/sdt:latest

# Or use the custom profile in docker-compose
docker-compose --profile custom up -d
```

## Building

### Build for Current Platform

```bash
# From project root
task docker:build

# Or manually
cd docker
docker build -t sandrolain/sdt:latest .
```

### Build Multi-Architecture Image

```bash
# From project root (requires buildx)
task docker:build:multi
```

This builds for both `linux/amd64` and `linux/arm64` platforms.

## Image Details

- **Base Image**: `lipanski/docker-static-website:2.4.0`
- **Size**: ~2MB (minimal static file server)
- **User**: Non-root (appuser)
- **Port**: 3000
- **Content**: SDT web interface (WASM-based)

## Environment Variables

Currently, the web interface doesn't require any environment variables as it runs entirely client-side.

## Volumes

No volumes are required. The web interface is fully static and client-side.

## Security

- Runs as non-root user (`appuser`)
- Minimal attack surface (static files only)
- No external dependencies at runtime
- All processing happens client-side in the browser

## Troubleshooting

### Port Already in Use

If port 3000 is already in use, change the host port:

```bash
docker run -p 8080:3000 sandrolain/sdt:latest
```

### Image Not Found

Pull the latest image:

```bash
docker pull sandrolain/sdt:latest
```

### Building Locally

Make sure you've built the web interface first:

```bash
task build:web
task docker:build
```

## Docker Hub

The image is available on Docker Hub: <https://hub.docker.com/r/sandrolain/sdt>

```bash
docker pull sandrolain/sdt:latest
docker pull sandrolain/sdt:v1.0.0  # Specific version
```
