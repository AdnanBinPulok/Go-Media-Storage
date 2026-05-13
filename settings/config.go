package settings

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/joho/godotenv"
)

var (
	globalAppInfo             AppInfo
	AppInfoInstance           AppInfo
	DatabaseConfigInstance    DatabaseConfig
	CachingConfigInstance     CachingConfig
	FileStorageConfigInstance FileStorageConfig
	loadOnce                  sync.Once
)

type CachingConfig struct {
	BrowserCacheTime int
	ProxyCacheTime   int
}

type AppInfo struct {
	APP_HOST  string
	APP_PORT  string
	LOG_LEVEL string
	DATABASE  DatabaseConfig
	BASE_URL  string
}

type DatabaseConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	Name         string
	SSLMode      string
	DSN          string // Data Source Name, e.g. "file:./data/media.db?_foreign_keys=on"
	MaxOpenConns int
	MaxIdleConns int
}

// LoadEnv loads environment variables from secrets/.env if it exists
// and returns the application host/port values.
func LoadEnv() AppInfo {
	loadOnce.Do(func() {
		loadDotEnv()
		dbConfig := databaseConfigFromEnv()
		globalAppInfo = AppInfo{
			APP_HOST:  getEnv("APP_HOST", "0.0.0.0"),
			APP_PORT:  getEnv("APP_PORT", "8432"),
			LOG_LEVEL: getEnv("LOG_LEVEL", "info"),
			DATABASE:  dbConfig,
			BASE_URL:  getEnv("BASE_URL", "http://localhost:8432"),
		}
		CachingConfigInstance = CachingConfig{
			BrowserCacheTime: getEnvInt("BROWSER_CACHE_TIME_SECONDS", 30), // Default 30 seconds for quick purging during development, can be increased in production
			ProxyCacheTime:   getEnvInt("PROXY_CACHE_TIME_SECONDS", 60),   // Default 60 seconds for faster purging when needed, can be increased in production
		}
		DatabaseConfigInstance = dbConfig
		FileStorageConfigInstance = GetFileStorageConfig()
		AppInfoInstance = globalAppInfo
	})

	return AppInfoInstance
}

func GetConfig() AppInfo {
	// Ensure the environment is loaded and globalAppInfo is initialized.
	return LoadEnv()
}

func (a AppInfo) ApiAddress() string {
	return a.APP_HOST + ":" + a.APP_PORT
}

func (a AppInfo) BaseURL() string {
	return a.BASE_URL
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return fallback
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return fallback
	}
	return value
}

func loadDotEnv() {
	_ = godotenv.Load("secrets/.env")
}

func databaseConfigFromEnv() DatabaseConfig {
	host := getEnv("POSTGRES_HOST", "127.0.0.1")
	port := getEnvInt("POSTGRES_PORT", 5432)
	user := getEnv("POSTGRES_USER", "postgres")
	password := getEnv("POSTGRES_PASSWORD", "postgres")
	name := getEnv("POSTGRES_DB", "go_media_storage")
	sslMode := getEnv("POSTGRES_SSL_MODE", "disable")

	dsn := getEnv("DATABASE_DSN", "")
	if dsn == "" {
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, name, sslMode)
	}

	return DatabaseConfig{
		Host:         host,
		Port:         port,
		User:         user,
		Password:     password,
		Name:         name,
		SSLMode:      sslMode,
		DSN:          dsn,
		MaxOpenConns: getEnvInt("DATABASE_MAX_OPEN_CONNS", 25),
		MaxIdleConns: getEnvInt("DATABASE_MAX_IDLE_CONNS", 10),
	}
}

type FileStorageConfig struct {
	BasePath          string
	StorageLimitBytes int64 // Maximum allowed storage in bytes (0 means no limit)
}

func GetFileStorageConfig() FileStorageConfig {
	return FileStorageConfig{
		BasePath:          getEnv("FILE_STORAGE_LOCAL_BASE_PATH", "./storage"),
		StorageLimitBytes: parseStorageLimit(getEnv("FILE_STORAGE_LIMIT", "0")), // Default to 0 (no limit) if not set
	}
}

// parseStorageLimit converts human-readable storage sizes (e.g., "1MB", "500MB", "2.5GB", "1TB") into bytes
func parseStorageLimit(val string) int64 {
	valStr := strings.TrimSpace(strings.ToUpper(val))
	if valStr == "" || valStr == "0" {
		return 0
	}

	// Find the index where the numeric part ends and the unit suffix begins
	idx := strings.IndexFunc(valStr, func(r rune) bool {
		return !unicode.IsDigit(r) && r != '.'
	})

	if idx == -1 {
		// Only numbers found, assume it's already in bytes
		bytes, err := strconv.ParseInt(valStr, 10, 64)
		if err != nil {
			return 0
		}
		return bytes
	}

	numStr := strings.TrimSpace(valStr[:idx])
	unitStr := strings.TrimSpace(valStr[idx:])

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}

	var multiplier float64 = 1
	switch unitStr {
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "PB", "P":
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
	case "B":
		multiplier = 1
	default:
		multiplier = 1
	}

	return int64(num * multiplier)
}
