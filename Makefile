.PHONY: build test lint release-snapshot help

help:
	@echo "Available targets:"
	@echo "  build              Build the binary"
	@echo "  test               Run tests"
	@echo "  lint               Run linters"
	@echo "  release-snapshot   Test release build (no publish)"

build:
	go build -o docker-agent-tail .

test:
	go test -race ./...

lint:
	golangci-lint run

release-snapshot:
	goreleaser release --snapshot --clean
