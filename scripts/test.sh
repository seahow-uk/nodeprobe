#!/bin/bash

set -e

echo "Running NodeProbe tests..."

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...

# Generate coverage report
go tool cover -html=coverage.out -o coverage.html

echo "Tests completed!"
echo "Coverage report: coverage.html" 