package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	cache "github.com/patrickmn/go-cache"
)

var siteDataCache = cache.New(30*time.Second, 2*time.Minute)

func GetSiteData() map[string]any {
	if cached, ok := siteDataCache.Get("site_data"); ok {
		if data, valid := cached.(map[string]any); valid {
			clonedData := cloneMap(data)
			clonedData["cached"] = true
			fmt.Printf("Cache Hit: %v\n", true)
			return clonedData
		}
	}

	data := make(map[string]any)

	bytes, err := os.ReadFile("data/site.json")
	if err != nil {
		return map[string]any{}
	}

	if err := json.Unmarshal(bytes, &data); err != nil {
		return map[string]any{}
	}

	siteDataCache.Set("site_data", data, cache.DefaultExpiration)

	clonedData := cloneMap(data)
	clonedData["cached"] = false
	fmt.Printf("Cache Miss: %v\n", true)
	return clonedData
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}

	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}

	return dst
}

func UpdateSiteData(newData map[string]any) error {
	if newData == nil {
		return fmt.Errorf("newData cannot be nil")
	}

	bytes, err := json.MarshalIndent(newData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal site data: %w", err)
	}

	if err := os.WriteFile("data/site.json", bytes, 0644); err != nil {
		return fmt.Errorf("write site data to file: %w", err)
	}

	siteDataCache.Set("site_data", newData, cache.DefaultExpiration)

	return nil
}

func GetWhitelistedEmails() []string {
	bytes, err := os.ReadFile("data/whitelist.json")
	if err != nil {
		return []string{}
	}

	var emails []string
	if err := json.Unmarshal(bytes, &emails); err != nil {
		return []string{}
	}

	return emails
}

func UpdateWhitelistedEmails(emails []string) error {
	if emails == nil {
		return fmt.Errorf("emails cannot be nil")
	}

	bytes, err := json.MarshalIndent(emails, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal whitelisted emails: %w", err)
	}

	if err := os.WriteFile("data/whitelist.json", bytes, 0644); err != nil {
		return fmt.Errorf("write whitelisted emails to file: %w", err)
	}

	return nil
}
