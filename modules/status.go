package modules

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"go-media-storage/services"
	"go-media-storage/settings"

	cache "github.com/patrickmn/go-cache"
)

var (
	statusDataCache = cache.New(5*time.Second, 10*time.Second)
	serverStartTime = time.Now()
)

// GetPublicStatusData returns a public-safe subset of the status data.
// Sensitive fields (env mode, goroutine count, storage totals, file type
// breakdown, service error messages and latency) are omitted.
func GetPublicStatusData() map[string]any {
	full := GetStatusData()

	// Strip sensitive service fields
	if rawSvcs, ok := full["services"].([]map[string]any); ok {
		safe := make([]map[string]any, 0, len(rawSvcs))
		for _, svc := range rawSvcs {
			safe = append(safe, map[string]any{
				"name":         svc["name"],
				"state":        svc["state"],
				"scope":        svc["scope"],
				"last_checked": svc["last_checked"],
				"detail":       svc["detail"],
			})
		}
		full["services"] = safe
	}

	// Strip sensitive runtime labels
	if rt, ok := full["runtime"].(map[string]any); ok {
		full["runtime"] = map[string]any{
			"uptime":     rt["uptime"],
			"go_version": rt["go_version"],
		}
	}

	// Remove fields not appropriate for public consumption
	delete(full, "mode")
	delete(full, "storage")
	delete(full, "file_type_counts")

	return full
}

// GetStatusData returns live operational data for the service. Results are
// cached for 30 seconds so repeated public requests are cheap.
func GetStatusData() map[string]any {
	if cached, ok := statusDataCache.Get("status_data"); ok {
		if data, valid := cached.(map[string]any); valid {
			result := cloneMap(data)
			result["cached"] = true
			return result
		}
	}

	now := time.Now().UTC()

	siteData := GetSiteData()
	version, _ := siteData["version"].(string)
	projectName, _ := siteData["title"].(string)
	if projectName == "" {
		projectName = "go-media-storage"
	}

	mode := settings.GetConfig().LOG_LEVEL
	if mode == "" {
		mode = "info"
	}

	svcs := []map[string]any{
		checkService("postgres", "database", pingPostgres),
		checkService("local-storage", "filesystem", pingStorage),
	}

	totalFiles, totalSizeBytes := queryStorageTotals()
	fileTypeCounts := queryFileTypeCounts()

	data := map[string]any{
		"generated_at": now.Format("2006-01-02T15:04:05Z"),
		"project_name": projectName,
		"mode":         mode,
		"version":      version,
		"services":     svcs,
		"runtime": map[string]any{
			"uptime":     formatUptime(time.Since(serverStartTime)),
			"go_version": runtime.Version(),
			"labels": map[string]string{
				"goroutines": fmt.Sprintf("%d", runtime.NumGoroutine()),
				"env":        mode,
			},
		},
		"storage": map[string]any{
			"total_files": totalFiles,
			"total_size":  formatBytes(totalSizeBytes),
		},
		"file_type_counts": fileTypeCounts,
		"endpoints": []map[string]any{
			{
				"name":        "Ping",
				"description": "Basic health probe for API readiness.",
				"status":      "ok",
				"method":      "GET",
				"path":        "/api/ping",
			},
			{
				"name":        "IP",
				"description": "Checks client IP and proxy forwarding behaviour.",
				"status":      "ok",
				"method":      "GET",
				"path":        "/api/ip",
			},
			{
				"name":        "Status",
				"description": "Operational status and metrics (this endpoint).",
				"status":      "ok",
				"method":      "GET",
				"path":        "/api/status",
			},
			{
				"name":        "Upload",
				"description": "Upload media files to a vault.",
				"status":      "ok",
				"method":      "POST",
				"path":        "/api/upload",
			},
			{
				"name":        "Uploads",
				"description": "List media files in a vault.",
				"status":      "ok",
				"method":      "GET",
				"path":        "/api/uploads",
			},
			{
				"name":        "Vaults",
				"description": "List vaults belonging to the authenticated user.",
				"status":      "ok",
				"method":      "GET",
				"path":        "/api/vaults",
			},
		},
		"cached": false,
	}

	statusDataCache.Set("status_data", data, cache.DefaultExpiration)
	return data
}

// checkService probes a dependency and returns a status map matching the
// services template structure.
func checkService(name, scope string, probe func() error) map[string]any {
	start := time.Now()
	err := probe()
	latency := time.Since(start).Milliseconds()

	state := "healthy"
	svc := map[string]any{
		"name":         name,
		"state":        state,
		"latency_ms":   latency,
		"last_checked": time.Now().UTC().Format("2006-01-02 15:04:05"),
		"scope":        scope,
	}

	if err != nil {
		svc["state"] = "degraded"
		svc["error"] = err.Error()
	} else {
		switch name {
		case "postgres":
			svc["detail"] = "Database connection is responding normally."
		case "local-storage":
			svc["detail"] = "Storage subsystem is responding normally."
		}
	}

	return svc
}

func pingPostgres() error {
	if services.DatabaseConn == nil {
		return fmt.Errorf("not initialized")
	}
	return services.DatabaseConn.Ping()
}

func pingStorage() error {
	_, err := os.Stat(settings.FileStorageConfigInstance.BasePath)
	return err
}

func queryStorageTotals() (int64, int64) {
	if services.DatabaseConn == nil {
		return 0, 0
	}
	rows, err := services.DatabaseConn.RunQuery(
		"SELECT COUNT(*) AS total_files, COALESCE(SUM(file_size_bytes), 0)::bigint AS total_size FROM media_files WHERE deleted = $1",
		[]any{false},
	)
	if err != nil || len(rows) == 0 {
		return 0, 0
	}
	files, _ := rows[0]["total_files"].(int64)
	size, _ := rows[0]["total_size"].(int64)
	return files, size
}

func queryFileTypeCounts() map[string]int64 {
	result := make(map[string]int64)
	if services.DatabaseConn == nil {
		return result
	}
	rows, err := services.DatabaseConn.RunQuery(
		"SELECT file_ext, COUNT(*) AS cnt FROM media_files WHERE deleted = $1 GROUP BY file_ext ORDER BY cnt DESC",
		[]any{false},
	)
	if err != nil {
		return result
	}
	for _, row := range rows {
		ext, _ := row["file_ext"].(string)
		cnt, _ := row["cnt"].(int64)
		if ext != "" {
			result[ext] = cnt
		}
	}
	return result
}

func formatUptime(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatBytes(b int64) string {
	if b <= 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
