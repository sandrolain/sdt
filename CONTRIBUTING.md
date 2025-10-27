# Contributing to SDT

Thank you for your interest in contributing to SDT! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and collaborative environment.

## Getting Started

### Prerequisites

- Go 1.22 or later
- [Task](https://taskfile.dev/) (recommended) or standard Go tools
- Git
- Docker (optional, for testing Docker builds)

### Setting Up Your Development Environment

1. Fork the repository on GitHub
2. Clone your fork locally:

   ```bash
   git clone https://github.com/YOUR_USERNAME/sdt.git
   cd sdt
   ```

3. Add the upstream repository:

   ```bash
   git remote add upstream https://github.com/sandrolain/sdt.git
   ```

4. Install dependencies:

   ```bash
   go mod download
   ```

5. Build the project:

   ```bash
   task build
   # or
   go build -o bin/sdt ./cli
   ```

## Development Workflow

### Creating a Branch

Always create a new branch for your work:

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

### Making Changes

1. Write clean, idiomatic Go code
2. Follow the existing code style
3. Add tests for new functionality
4. Update documentation as needed

### Running Tests

```bash
# Run all tests
task test

# Run tests with coverage
task test:coverage

# Run tests with race detector
task test:race

# Run benchmarks
task test:bench
```

### Code Quality Checks

Before committing, ensure your code passes all checks:

```bash
# Run all checks
task check

# Or run individual checks
task fmt      # Format code
task lint     # Run linter
task vet      # Run go vet
task gosec    # Security scan
```

### Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/) format:

```text
<type>(<scope>): <subject>

<body>

<footer>
```

Types:

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks
- `perf`: Performance improvements

Examples:

```bash
feat(jwt): add JWT validation command
fix(b64): handle empty input correctly
docs(readme): update installation instructions
test(hash): add tests for SHA-256 function
```

### Documentation

- Update inline documentation (comments) for new code
- Update README.md if adding new features
- Run `task docs` to regenerate CLI documentation

## Pull Request Process

1. **Update your fork** with the latest upstream changes:

   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Push your changes** to your fork:

   ```bash
   git push origin feature/your-feature-name
   ```

3. **Create a Pull Request** on GitHub with:
   - Clear title following conventional commits format
   - Description of changes
   - Related issue numbers (if applicable)
   - Screenshots (if UI changes)

4. **Ensure CI passes**:
   - All tests pass
   - Code coverage is maintained or improved
   - Linting passes
   - Security scans pass

5. **Respond to feedback** from reviewers

6. Once approved, a maintainer will merge your PR

## Guidelines

### Code Style

- Follow Go best practices and idioms
- Use `gofmt` for formatting (run `task fmt`)
- Keep functions small and focused
- Add comments for exported functions and types
- Use meaningful variable and function names

### Testing

- Write unit tests for new functionality
- Aim for at least 80% code coverage
- Test edge cases and error conditions
- Use table-driven tests where appropriate

Example test structure:

```go
func TestFeatureName(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:  "valid input",
            input: "test",
            want:  "expected",
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := YourFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Documentation

- Document all exported functions, types, and constants
- Use complete sentences in comments
- Provide usage examples in documentation
- Keep README.md up to date

## Adding New Commands

When adding a new command to the CLI:

1. Create the command file in `cli/cmd/`
2. Implement the command logic
3. Add tests in `cli/cmd/*_test.go`
4. Run `task docs` to update documentation
5. Update README.md with the new command in the appropriate category

## Questions or Need Help?

- Open an issue for bugs or feature requests
- Start a discussion for questions or ideas
- Check existing issues and PRs to avoid duplicates

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

## Recognition

Contributors will be recognized in the project's README and release notes.

Thank you for contributing to SDT! 🎉
