#!/bin/bash

set -e

echo "Building NodeProbe..."

# Clean previous builds
rm -f nodeprobe

# Build the application
CGO_ENABLED=1 go build -ldflags="-w -s" -o nodeprobe ./cmd/nodeprobe

echo "Build completed successfully!"
echo "Binary: ./nodeprobe" 