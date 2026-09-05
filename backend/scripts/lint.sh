#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root_dir}"

echo "Checking formatting"
unformatted="$(gofmt -l .)"
if [[ -n "${unformatted}" ]]; then
  echo "Files are not formatted:"
  echo "${unformatted}"
  echo "Run: gofmt -w ."
  exit 1
fi

echo "Running go vet"
go vet ./...

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "golangci-lint is not installed."
  echo "Install the pinned version with:"
  echo "go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8"
  exit 1
fi

echo "Running golangci-lint"
golangci-lint run ./...
