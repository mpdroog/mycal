.PHONY: all build run clean lint lint-fix lint-js lint-js-fix test test-cover deps check

BINARY=mycal
CGO_ENABLED=0
GOBIN=$(shell go env GOPATH)/bin

all: lint lint-js test build

build:
	CGO_ENABLED=$(CGO_ENABLED) go build -o $(BINARY) .

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
	rm -rf data/

lint:
	$(GOBIN)/golangci-lint run --config .golangci.yml

lint-fix:
	$(GOBIN)/golangci-lint run --config .golangci.yml --fix

lint-js:
	npx eslint 'static/js/**/*.js' --ignore-pattern '*.min.js'

lint-js-fix:
	npx eslint 'static/js/**/*.js' --ignore-pattern '*.min.js' --fix

test:
	go test -v ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

deps:
	go mod download
	go mod tidy

check: lint lint-js test
	@echo "All checks passed!"

install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
