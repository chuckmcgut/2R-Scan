#!/bin/bash
# run-test.sh — Run all tests for 2R-Scan server

set -e

echo "=== 2R-Scan Server Test Suite ==="

if ! command -v go &> /dev/null; then
    echo "ERROR: Go is not installed"
    exit 1
fi

cd "$(dirname "$0")/.."

echo "Go version: $(go version)"
echo ""

echo "--- Running all tests ---"
go test ./... -v -race

echo ""
echo "--- Running carddb tests ---"
go test ./internal/carddb/... -v

echo ""
echo "--- Running grading tests ---"
go test ./internal/grading/... -v

echo ""
echo "--- Running go vet ---"
go vet ./...

echo ""
echo "=== All tests passed ==="