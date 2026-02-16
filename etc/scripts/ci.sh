#!/usr/bin/env bash
set -e

cleanup() {
    local exit_code=$?
    # Generate reports even if tests fail
    go tool cover -html=coverage.out -o "$CI_PROJECT_DIR/coverage.html" 2>/dev/null || true
    gocover-cobertura < "$CI_PROJECT_DIR/coverage.out" > "$CI_PROJECT_DIR/coverage.xml" 2>/dev/null || true
    exit $exit_code
}
trap cleanup EXIT

cd "$1" || exit
go install github.com/boumenot/gocover-cobertura@latest
go vet ./...
go build -v ./cmd/goimapnotify
go test -race -coverprofile="$CI_PROJECT_DIR/coverage.out" -covermode=atomic ./...
