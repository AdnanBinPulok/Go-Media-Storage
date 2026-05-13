#!/bin/bash
# update.sh

# Pull latest changes
printf "\n\nPulling latest changes...\n\n"
git pull
printf "\n\nGit pull successful.\n\n"

# Download Go modules
if [ -f "go.mod" ]; then
    go mod download
    printf "\n\nGo modules downloaded successfully.\n\n"
fi

# Build the application
printf "\n\nBuilding the application...\n\n"
go build -o go-media-storage main.go

if [ $? -ne 0 ]; then
    printf "\n\nBuild failed. Exiting.\n\n"
    exit 1
fi
printf "\n\nBuild successful.\n\n"

printf "\n\nRestarting the application using pm2...\n\n"
# Restart with pm2
pm2 restart "GO_MEDIA_STORAGE" & pm2 logs "GO_MEDIA_STORAGE" --lines 100
printf "\n\nApplication updated and restarted successfully.\n\n"
