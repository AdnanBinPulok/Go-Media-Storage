package routes

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"go-media-storage/settings"
	"go-media-storage/utils"

	"github.com/gofiber/fiber/v2"
)

func newAvatarAPIApp() *fiber.App {
	app := fiber.New()
	AvatarRoutes(app)
	return app
}

func getRandomAvatar(t *testing.T, app *fiber.App, query string) (int, map[string]any) {
	t.Helper()

	path := "/avatar/random"
	if query != "" {
		path += "?" + query
	}

	req := httptest.NewRequest("GET", path, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var payload map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("json: %v\nbody: %s", err, body)
		}
	}
	return resp.StatusCode, payload
}

func TestRandomAvatarReturnsURL(t *testing.T) {
	originalBase := settings.AppInfoInstance.BASE_URL
	originalSize := settings.AvatarConfigInstance.DefaultSize
	t.Cleanup(func() {
		settings.AppInfoInstance.BASE_URL = originalBase
		settings.AvatarConfigInstance.DefaultSize = originalSize
	})

	settings.AppInfoInstance.BASE_URL = "https://vault.gliders.dev"
	settings.AvatarConfigInstance.DefaultSize = 256

	app := newAvatarAPIApp()
	status, body := getRandomAvatar(t, app, "")

	if status != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %v, want ok", body["status"])
	}

	id, _ := body["uuid"].(string)
	if _, ok := utils.NormalizeUUID(id); !ok {
		t.Fatalf("uuid %q is not a valid UUID", id)
	}

	size, ok := body["size"].(float64) // JSON numbers decode as float64
	if !ok || int(size) != 256 {
		t.Errorf("size = %v, want 256", body["size"])
	}

	url, _ := body["url"].(string)
	want := "https://vault.gliders.dev/avatar/" + id + "?size=256"
	if url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
}

func TestRandomAvatarUniqueEachCall(t *testing.T) {
	settings.AppInfoInstance.BASE_URL = "http://localhost:8432"
	app := newAvatarAPIApp()

	_, a := getRandomAvatar(t, app, "")
	_, b := getRandomAvatar(t, app, "")

	if a["uuid"] == b["uuid"] {
		t.Error("two successive calls returned the same UUID")
	}
}

func TestRandomAvatarSizeQuery(t *testing.T) {
	settings.AppInfoInstance.BASE_URL = "http://localhost:8432"
	settings.AvatarConfigInstance.DefaultSize = 256
	app := newAvatarAPIApp()

	status, body := getRandomAvatar(t, app, "size=128")
	if status != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if int(body["size"].(float64)) != 128 {
		t.Errorf("size = %v, want 128", body["size"])
	}
	url, _ := body["url"].(string)
	if !strings.Contains(url, "?size=128") {
		t.Errorf("url = %q, missing ?size=128", url)
	}
}

func TestRandomAvatarClampsSize(t *testing.T) {
	settings.AppInfoInstance.BASE_URL = "http://localhost:8432"
	app := newAvatarAPIApp()

	_, body := getRandomAvatar(t, app, "size=99999")
	if int(body["size"].(float64)) != 4096 {
		t.Errorf("size = %v, want 4096", body["size"])
	}

	_, body = getRandomAvatar(t, app, "size=1")
	if int(body["size"].(float64)) != 8 {
		t.Errorf("size = %v, want 8", body["size"])
	}
}

func TestRandomAvatarRejectsInvalidSize(t *testing.T) {
	app := newAvatarAPIApp()
	status, body := getRandomAvatar(t, app, "size=abc")
	if status != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
	if body["status"] != "error" {
		t.Errorf("status field = %v, want error", body["status"])
	}
}

func TestRandomAvatarNoStore(t *testing.T) {
	settings.AppInfoInstance.BASE_URL = "http://localhost:8432"
	app := newAvatarAPIApp()

	req := httptest.NewRequest("GET", "/avatar/random", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestAvatarURLTrimsTrailingSlash(t *testing.T) {
	got := avatarURL("550e8400-e29b-41d4-a716-446655440000", 64)
	// Depends on current BaseURL; just assert the helper shape with a forced base.
	original := settings.AppInfoInstance.BASE_URL
	t.Cleanup(func() { settings.AppInfoInstance.BASE_URL = original })

	settings.AppInfoInstance.BASE_URL = "https://example.com/"
	got = avatarURL("550e8400-e29b-41d4-a716-446655440000", 64)
	want := "https://example.com/avatar/550e8400-e29b-41d4-a716-446655440000?size=64"
	if got != want {
		t.Errorf("avatarURL = %q, want %q", got, want)
	}
}
