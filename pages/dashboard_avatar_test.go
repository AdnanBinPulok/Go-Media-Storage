package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
)

// TestDashboardAvatarTemplateRenders parses and executes the avatar dashboard
// templates. Go templates only report a missing {{template "..."}} or a broken
// field at execution time, so rendering once is the cheapest real check.
func TestDashboardAvatarTemplateRenders(t *testing.T) {
	engine := html.New("../static", ".html")
	if err := engine.Load(); err != nil {
		t.Fatalf("load templates: %v", err)
	}

	data := fiber.Map{
		"site": map[string]any{
			"title":    "Test Site",
			"favicon":  "/assets/favicon.ico",
			"base_url": "localhost:3000",
		},
		"user": map[string]any{
			"full_name": "Test User",
			"email":     "test@example.com",
		},
	}

	var buf bytes.Buffer
	if err := engine.Render(&buf, "pages/dashboard_avatar", data); err != nil {
		t.Fatalf("render: %v", err)
	}

	out := buf.String()

	for _, want := range []string{
		"avatar-form",
		"avatar-uuid",
		`min="8" max="4096"`,
		"avatar-size-presets",
		"avatar-image",
		"avatar-gallery",
		"shuffleAvatarGallery",
		"/dashboard/avatar", // active sidebar entry
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered page is missing %q", want)
		}
	}

	// The size input must default to 256, not to the old 128.
	if !strings.Contains(out, `value="256"`) {
		t.Error("size input does not default to 256")
	}
}
