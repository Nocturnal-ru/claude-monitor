#!/bin/bash
set -e

echo "=== Claude Monitor — Build ==="

if ! command -v go &> /dev/null; then
    echo "ERROR: Go not installed. Run: sudo dnf install golang"
    exit 1
fi

echo "Go: $(go version)"
echo ""

echo "-> Downloading dependencies..."
go mod tidy
echo ""

echo "-> Building for Windows (amd64)..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o claude-monitor.exe .

if [ -f "claude-monitor.exe" ]; then
    SIZE=$(du -h claude-monitor.exe | cut -f1)
    echo ""
    echo "OK! claude-monitor.exe ($SIZE)"
    echo ""
    echo "Copy claude-monitor.exe to Windows and run it."
else
    echo "Build failed!"
    exit 1
fi
