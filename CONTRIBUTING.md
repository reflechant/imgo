# Contributing to ImGo

Thank you for your interest in ImGo! As a language that values deep immutability and functional purity, we have strict guidelines to ensure the core remains safe and consistent.

## Development Workflow

1.  **Spec First:** All language changes must be reflected in `docs/SPEC.md` first.
2.  **100% Coverage:** The transpiler core (`rewrite.go` and `validator.go`) **must** maintain 100% statement coverage.
3.  **Integration Tests:** Every new feature must include a corresponding `.im` fixture in `pkg/transpiler/testdata/` with expected output markers.

## Setting Up Your Environment

```bash
# Install Go 1.23+
# Clone the repository
git clone https://github.com/rg/imgo.git
cd imgo

# Build the tool
go build -o imgo ./cmd/imgo

# Run tests
go test ./...
```

## Pull Request Process

- Ensure all tests pass.
- Ensure formatting is correct (`gofmt -s -w .`).
- Update `CHANGELOG.md` with your changes under the `[Unreleased]` section (or create a new version section if appropriate).
- Maintain the "Functional Core, Imperative Shell" philosophy.

## Coding Standards

- Avoid `var` in function blocks (use `:=`).
- Never introduce mutation where a functional transformation is possible.
- Use SSA-style mangling for new transformation patterns.

## 🤖 AI Assistance
Contributors are encouraged to use AI tools responsibly. This project is maintained with the assistance of Claude 4.6, 4.7, and Gemini 3.
