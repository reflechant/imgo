.PHONY: lint test check-coverage validate clean build fmt

# Run all validation steps
validate: lint build test check-coverage

# Build the imgo binary
build:
	go build -v -o imgo ./cmd/imgo

# Format Go files
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run --fix

# Run tests
test:
	go test -v -race -count=1 -coverprofile=coverage.out ./...

# Verify coverage gates
check-coverage:
	./scripts/check_coverage.sh

# Clean build artifacts
clean:
	rm -f imgo coverage.out
