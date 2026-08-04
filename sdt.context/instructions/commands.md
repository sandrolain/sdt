# Build, Test and Lint

Commands an agent must run to build, test, lint, vet and format this project.
Keep this file minimal and accurate.

```
go build -o bin/sdt ./cli                          # build the CLI binary
go test ./...                                      # run all tests (≥80% coverage)
go test ./... -coverprofile=coverage.out           # coverage report
go tool cover -func=coverage.out
golangci-lint run ./...                            # lint (0 issues)
govulncheck ./...                                  # vulnerability check
gofmt -w ./cli/...                                 # format code
```

Whenever these commands change, update this file (see self-update.md).
