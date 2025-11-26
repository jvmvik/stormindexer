.PHONY: build clean run test test-verbose test-coverage test-race test-package test-models test-database test-indexer test-sync test-config install fmt vet help

BINARY_NAME=stormindexer
GO=go

build:
	$(GO) build -o $(BINARY_NAME) .

clean:
	rm -f $(BINARY_NAME)
	rm -f *.db
	rm -f *.sqlite
	rm -f *.sqlite3
	rm -f coverage.out
	rm -f coverage.html

run: build
	./$(BINARY_NAME)

test:
	$(GO) test ./...

test-verbose:
	$(GO) test ./... -v

test-coverage:
	$(GO) test ./... -cover
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race:
	$(GO) test ./... -race

test-package:
	@echo "Available packages:"
	@echo "  make test-models     - Test models package"
	@echo "  make test-database   - Test database package"
	@echo "  make test-indexer    - Test indexer package"
	@echo "  make test-sync       - Test sync package"
	@echo "  make test-config     - Test config package"

test-models:
	$(GO) test ./internal/models/... -v

test-database:
	$(GO) test ./internal/database/... -v

test-indexer:
	$(GO) test ./internal/indexer/... -v

test-sync:
	$(GO) test ./internal/sync/... -v

test-config:
	$(GO) test ./internal/config/... -v

install:
	$(GO) install .

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

help:
	@echo "StormIndexer Makefile Commands:"
	@echo ""
	@echo "Build:"
	@echo "  make build          - Build the binary"
	@echo "  make install        - Install the binary"
	@echo "  make run            - Build and run the binary"
	@echo ""
	@echo "Testing:"
	@echo "  make test           - Run all tests"
	@echo "  make test-verbose  - Run all tests with verbose output"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make test-race      - Run tests with race detector"
	@echo "  make test-package   - Show available package-specific test commands"
	@echo ""
	@echo "Package-specific tests:"
	@echo "  make test-models    - Test models package"
	@echo "  make test-database  - Test database package"
	@echo "  make test-indexer   - Test indexer package"
	@echo "  make test-sync      - Test sync package"
	@echo "  make test-config    - Test config package"
	@echo ""
	@echo "Code Quality:"
	@echo "  make fmt            - Format code"
	@echo "  make vet            - Run go vet"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean          - Remove build artifacts and databases"

