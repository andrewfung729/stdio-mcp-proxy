#!/bin/sh
set -eu
cd "$(dirname "$0")"
go build -o stdio-mcp-proxy ./cmd/stdio-mcp-proxy
echo "built $(pwd)/stdio-mcp-proxy"
