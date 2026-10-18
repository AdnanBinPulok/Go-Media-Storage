package pages

import (
	"bytes"
	"image/png"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

const testUUID = "550e8400-e29b-41d4-a716-446655440000"

func newAvatarApp() *fiber.App {
	app := fiber.New()
	AvatarPage(app)
	return app
}

func TestAvatarRouteHeadersAndCaching(t *testing.T) {
	app := newAvatarApp()

	req := httptest.NewRequest("GET", "/avatar/"+testUUID, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	if got := resp.Header.Get("Cache-Control"); !strings.Contains(got, "public") ||
		!strings.Contains(got, "max-age=86400") || !strings.Contains(got, "immutable") {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := resp.Header.Get("Cloudflare-CDN-Cache-Control"); !strings.Contains(got, "max-age=86400") {
		t.Errorf("Cloudflare-CDN-Cache-Control = %q", got)
	}
	if got := resp.Header.Get("CDN-Cache-Control"); !strings.Contains(got, "max-age=86400") {
		t.Errorf("CDN-Cache-Control = %q", got)
	}
	if etag := resp.Header.Get("ETag"); !strings.HasPrefix(etag, `"`) {
		t.Errorf("ETag = %q, want a quoted strong validator", etag)
	}
	if got := resp.Header.Get("X-Avatar-Cache"); got != "MISS" {
		t.Errorf("X-Avatar-Cache = %q, want MISS on first request", got)
	}

	// Second request must come from the in-memory cache.
	req2 := httptest.NewRequest("GET", "/avatar/"+testUUID, nil)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("second GET: %v", err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()

	if got := resp2.Header.Get("X-Avatar-Cache"); got != "HIT" {
		t.Errorf("X-Avatar-Cache = %q, want HIT on second request", got)
	}
	if !bytes.Equal(body, body2) {
		t.Error("cached response differs from the generated one")
	}
}

func TestAvatarRouteNormalizesUUID(t *testing.T) {
	app := newAvatarApp()

	want := fetchBody(t, app, "/avatar/"+testUUID)

	variants := []string{
		"/avatar/550E8400-E29B-41D4-A716-446655440000",
		"/avatar/550e8400e29b41d4a716446655440000",
		"/avatar/%7B" + testUUID + "%7D",
		"/avatar/urn:uuid:" + testUUID,
		"/avatar/" + testUUID + "?size=256",
	}

	for _, target := range variants {
		got := fetchBody(t, app, target)
		if !bytes.Equal(got, want) {
			t.Errorf("%s produced a different avatar than the canonical form", target)
		}
	}
}

func TestAvatarRouteRejectsInvalidUUID(t *testing.T) {
	app := newAvatarApp()

	// The route is registered with a single :uuid segment, so anything without
	// a slash reaches the handler.
	for _, target := range []string{
		"/avatar/not-a-uuid",
		"/avatar/user_42",
		"/avatar/123",
		"/avatar/" + testUUID + "0",
	} {
		resp, err := app.Test(httptest.NewRequest("GET", target, nil), -1)
		if err != nil {
			t.Fatalf("GET %s: %v", target, err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode != fiber.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, resp.StatusCode)
		}
	}
}

func TestAvatarRouteRejectsInvalidSize(t *testing.T) {
	app := newAvatarApp()

	resp, err := app.Test(httptest.NewRequest("GET", "/avatar/"+testUUID+"?size=abc", nil), -1)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAvatarRouteSizes(t *testing.T) {
	app := newAvatarApp()

	cases := []struct {
		target string
		want   int
	}{
		{"/avatar/" + testUUID, 256},                  // default
		{"/avatar/" + testUUID + "?size=8", 8},        // min
		{"/avatar/" + testUUID + "?size=64", 64},      // normal
		{"/avatar/" + testUUID + "?size=4096", 4096},  // max
	}

	for _, tc := range cases {
		body := fetchBody(t, app, tc.target)

		img, err := png.Decode(bytes.NewReader(body))
		if err != nil {
			t.Fatalf("%s: decode: %v", tc.target, err)
		}
		if img.Bounds().Dx() != tc.want || img.Bounds().Dy() != tc.want {
			t.Errorf("%s: got %dx%d, want %dx%d", tc.target,
				img.Bounds().Dx(), img.Bounds().Dy(), tc.want, tc.want)
		}
	}
}

func TestAvatarRouteNotModified(t *testing.T) {
	app := newAvatarApp()

	first, err := app.Test(httptest.NewRequest("GET", "/avatar/"+testUUID+"?size=64", nil), -1)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	etag := first.Header.Get("ETag")
	io.Copy(io.Discard, first.Body)
	first.Body.Close()

	if etag == "" {
		t.Fatal("no ETag on the first response")
	}

	req := httptest.NewRequest("GET", "/avatar/"+testUUID+"?size=64", nil)
	req.Header.Set("If-None-Match", etag)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("conditional GET: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != fiber.StatusNotModified {
		t.Fatalf("status = %d, want 304", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Errorf("304 carried a %d byte body", len(body))
	}

	if got := resp.Header.Get("X-Avatar-Cache"); got != "HIT" {
		t.Errorf("X-Avatar-Cache = %q on revalidation, want HIT", got)
	}
}

func TestAvatarRouteETagVariesWithSize(t *testing.T) {
	app := newAvatarApp()

	small := fetchHeader(t, app, "/avatar/"+testUUID+"?size=64", "ETag")
	large := fetchHeader(t, app, "/avatar/"+testUUID+"?size=128", "ETag")

	if small == large {
		t.Error("ETag does not vary with size")
	}
}

func fetchBody(t *testing.T, app *fiber.App, target string) []byte {
	t.Helper()

	resp, err := app.Test(httptest.NewRequest("GET", target, nil), -1)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s: read body: %v", target, err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("%s: status = %d, body = %s", target, resp.StatusCode, body)
	}
	return body
}

func fetchHeader(t *testing.T, app *fiber.App, target, header string) string {
	t.Helper()

	resp, err := app.Test(httptest.NewRequest("GET", target, nil), -1)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return resp.Header.Get(header)
}

func BenchmarkAvatarRouteCached(b *testing.B) {
	app := newAvatarApp()
	target := "/avatar/" + testUUID + "?size=64"

	if resp, err := app.Test(httptest.NewRequest("GET", target, nil), -1); err != nil {
		b.Fatal(err)
	} else {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := app.Test(httptest.NewRequest("GET", target, nil), -1)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
