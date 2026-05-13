#!/bin/bash
# start.sh

# Download Go modules
if [ -f "go.mod" ]; then
    go mod download
    printf "\n\nGo modules downloaded successfully.\n\n"
fi

# Build the application
printf "\n\nBuilding the application...\n\n"
# Windows users might need .exe extension if running in Git Bash, but for standard linux/wsl usage:
go build -o go-media-storage main.go

if [ $? -ne 0 ]; then
    printf "\n\nBuild failed. Exiting.\n\n"
    exit 1
fi
printf "\n\nBuild successful.\n\n"

printf "\n\nStarting the application using pm2...\n\n"
# Start with pm2
pm2 start ./go-media-storage --name="GO_MEDIA_STORAGE" & pm2 logs "GO_MEDIA_STORAGE" --lines 100
printf "\n\nApplication started successfully with pm2.\n\n"
