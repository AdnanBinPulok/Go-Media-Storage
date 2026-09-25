#!/bin/bash
# start.sh — build + pm2 start (requires Go 1.25.1+)

# Allow Go 1.21+ installs to download the go.mod toolchain (go1.25.1) when needed.
export GOTOOLCHAIN=auto

if ! command -v go >/dev/null 2>&1; then
    printf "\nError: 'go' not found in PATH.\n"
    printf "Install Go 1.25.1 or newer, then retry.\n\n"
    exit 1
fi

printf "\nGo toolchain: %s\n" "$(go version)"
printf "Minimum required: Go 1.25.1 (or auto-downloaded toolchain via GOTOOLCHAIN=auto).\n\n"

# Download Go modules
if [ -f "go.mod" ]; then
    go mod download
    if [ $? -ne 0 ]; then
        printf "\n\nGo module download failed. Exiting.\n\n"
        exit 1
    fi
    printf "\n\nGo modules downloaded successfully.\n\n"
fi

# Build the application
printf "\n\nBuilding the application...\n\n"
# Windows users might need .exe extension if running in Git Bash, but for standard linux/wsl usage:
go build -o go-media-storage main.go

if [ $? -ne 0 ]; then
    printf "\n\nBuild failed. Exiting.\n\n"
    printf "This project requires Go 1.25.1 or newer. Air-gapped hosts must install that version locally.\n\n"
    exit 1
fi
printf "\n\nBuild successful.\n\n"

printf "\n\nStarting the application using pm2...\n\n"
# Start with pm2
pm2 start ./go-media-storage --name="GO_MEDIA_STORAGE" & pm2 logs "GO_MEDIA_STORAGE" --lines 100
printf "\n\nApplication started successfully with pm2.\n\n"
