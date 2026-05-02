.PHONY: all build build-js run clean lint lint-fix lint-js lint-js-fix lint-css typecheck test test-cover deps check install-tools

BINARY=mycal
CGO_ENABLED=0
GOBIN=$(shell go env GOPATH)/bin

all: lint lint-js lint-css typecheck test build

# Go build
build: build-js
	CGO_ENABLED=$(CGO_ENABLED) go build -o $(BINARY) .

# TypeScript build (minified with esbuild)
build-js:
	npm run build

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
	rm -rf data/
	rm -f static/js/dashboard.js static/js/food-form.js

# Go linting
lint:
	$(GOBIN)/golangci-lint run --config .golangci.yml

lint-fix:
	$(GOBIN)/golangci-lint run --config .golangci.yml --fix

# TypeScript/ESLint
lint-js:
	npx eslint 'static/js/src/**/*.ts'

lint-js-fix:
	npx eslint 'static/js/src/**/*.ts' --fix

# CSS linting
lint-css:
	npx stylelint 'static/css/**/*.css' --ignore-pattern '*.min.css'

lint-css-fix:
	npx stylelint 'static/css/**/*.css' --ignore-pattern '*.min.css' --fix

# TypeScript type checking
typecheck:
	npx tsc --noEmit

# Testing
test:
	go test -v ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Dependencies
deps:
	go mod download
	go mod tidy
	npm install

check: lint lint-js lint-css typecheck test
	@echo "All checks passed!"

install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	npm install
