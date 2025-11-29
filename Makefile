.PHONY: build test clean lint fmt

# Build the application
build:
	go build -o build/burndown ./cmd/burndown

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf build/

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Run linter and fix issues
lint-fix:
	golangci-lint run --fix

# Install dependencies
deps:
	go mod download
	go mod tidy

# All checks
check: fmt lint test build