package services

import (
	"go-media-storage/settings"
	"log"
	"strings"
)

func SetupLogger(appInfo settings.AppInfo) {
	level := strings.ToLower(strings.TrimSpace(appInfo.LOG_LEVEL))

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	switch level {
	case "debug":
		log.SetPrefix("[DEBUG] ")
	case "warn":
		log.SetPrefix("[WARN] ")
	case "error":
		log.SetPrefix("[ERROR] ")
	default:
		log.SetPrefix("[INFO] ")
	}
}
