package pages

import (
	"bytes"
	"strings"
	"testing"

	"go-media-storage/settings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
)

func renderAvatarDashboard(t *testing.T) string {
	t.Helper()

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
		"avatar": avatarTemplateData(),
	}

	var buf bytes.Buffer
	if err := engine.Render(&buf, "pages/dashboard_avatar", data); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

// TestDashboardAvatarTemplateRenders parses and executes the avatar dashboard
// templates. Go templates only report a missing {{template "..."}} or a broken
// field at execution time, so rendering once is the cheapest real check.
func TestDashboardAvatarTemplateRenders(t *testing.T) {
	out := renderAvatarDashboard(t)

	for _, want := range []string{
		"avatar-form",
		"avatar-uuid",
		"avatar-size-presets",
		"avatar-image",
		"avatar-gallery",
		"shuffleAvatarGallery",
		"/dashboard/avatar", // active sidebar entry
		`data-default-size="256"`,
		`data-min-size="8"`,
		`data-max-size="4096"`,
		`value="256"`,
		`data-size="4096"`, // presets are rendered from the server slice
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered page is missing %q", want)
		}
	}

	// The compact layout must not regress into full-width stacked sections.
	if !strings.Contains(out, "xl:grid-cols-") {
		t.Error("dashboard is not using the compact multi-column layout")
	}
}

// TestAvatarDefaultSizeFromEnv proves AVATAR_DEFAULT_SIZE reaches both the route
// and the rendered page, and that out-of-range values are clamped.
func TestAvatarDefaultSizeFromEnv(t *testing.T) {
	original := settings.AvatarConfigInstance.DefaultSize
	t.Cleanup(func() { settings.AvatarConfigInstance.DefaultSize = original })

	cases := []struct {
		name string
		env  int
		want int
	}{
		{"default", 256, 256},
		{"custom", 512, 512},
		{"below minimum", 1, 8},
		{"above maximum", 9999, 4096},
		{"unset", 0, 256},
	}

	for _, tc := range cases {
		settings.AvatarConfigInstance.DefaultSize = tc.env

		if got := avatarDefaultSize(); got != tc.want {
			t.Errorf("%s: avatarDefaultSize() = %d, want %d", tc.name, got, tc.want)
		}

		// The route must use the same default when ?size is absent.
		app := newAvatarApp()
		body := fetchBody(t, app, "/avatar/"+testUUID)

		img, err := pngSize(body)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if img != tc.want {
			t.Errorf("%s: route rendered %dpx, want %dpx", tc.name, img, tc.want)
		}
	}
}

// TestDashboardUsesConfiguredDefaultSize checks the page input reflects the env.
func TestDashboardUsesConfiguredDefaultSize(t *testing.T) {
	original := settings.AvatarConfigInstance.DefaultSize
	t.Cleanup(func() { settings.AvatarConfigInstance.DefaultSize = original })

	settings.AvatarConfigInstance.DefaultSize = 512

	out := renderAvatarDashboard(t)
	if !strings.Contains(out, `data-default-size="512"`) {
		t.Error("page did not receive the configured default size")
	}
	if !strings.Contains(out, `value="512"`) {
		t.Error("size input did not receive the configured default size")
	}
}
