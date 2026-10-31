package classes

import "testing"

func TestValidateOriginalFilenameRejectsTraversal(t *testing.T) {
	bad := []string{
		"",
		"poc.%2f%2e%2e%2f%2e%2e%2fsecrets%2f%2eenv",
		"../../etc/passwd",
		`..\..\windows\system32`,
		"a/b.jpg",
		"a\\b.jpg",
		"photo..jpg/../x",
		"x\x00.jpg",
		stringsRepeat("a", 300),
	}
	for _, name := range bad {
		if err := ValidateOriginalFilename(name); err == nil {
			t.Errorf("ValidateOriginalFilename(%q) = nil, want error", name)
		}
	}

	good := []string{
		"photo.jpg",
		"my file.webp",
		"archive.tar.gz",
		"Screenshot%202026-10-03%20105638.png", // bare percent-escaped spaces
		"100%done.jpg",                         // malformed escape is literal text
	}
	for _, name := range good {
		if err := ValidateOriginalFilename(name); err != nil {
			t.Errorf("ValidateOriginalFilename(%q) = %v, want nil", name, err)
		}
	}
}

func TestNormalizeOriginalFilenameDecodesPercent(t *testing.T) {
	cases := map[string]string{
		"Screenshot%202026-10-03%20105638.png": "Screenshot 2026-10-03 105638.png",
		"photo.jpg":                            "photo.jpg",
		"100%done.jpg":                         "100%done.jpg",
		"  padded.png  ":                       "padded.png",
	}
	for in, want := range cases {
		got, err := NormalizeOriginalFilename(in)
		if err != nil {
			t.Errorf("NormalizeOriginalFilename(%q) = %v, want %q", in, err, want)
			continue
		}
		if got != want {
			t.Errorf("NormalizeOriginalFilename(%q) = %q, want %q", in, got, want)
		}
	}

	// Decoding must not create an escape hatch: these still fail.
	bad := []string{
		"poc.%2f%2e%2e%2fsecrets%2f%2eenv",
		"poc.%252f%252e%252e%252f.env",
		"%25252e%25252e",
		"a%2Fb.jpg",
		"..%2Fetc%2Fpasswd",
	}
	for _, in := range bad {
		if got, err := NormalizeOriginalFilename(in); err == nil {
			t.Errorf("NormalizeOriginalFilename(%q) = %q, want error", in, got)
		}
	}
}

func stringsRepeat(s string, n int) string {
	b := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}

func TestSafeStoredExt(t *testing.T) {
	cases := []struct {
		name, mime, want string
	}{
		{"photo.JPG", "image/jpeg", ".jpg"},
		{"poc.%2f%2e%2e%2f.env", "image/png", ".png"}, // .env not allowed → mime fallback
		{"secret.env", "text/plain", ".txt"},
		{"noext", "image/webp", ".webp"},
		{"file.unknownexttoolonggg", "application/octet-stream", ""},
		{"data.json", "application/json", ".json"},
	}
	for _, tc := range cases {
		if got := SafeStoredExt(tc.name, tc.mime); got != tc.want {
			t.Errorf("SafeStoredExt(%q, %q) = %q, want %q", tc.name, tc.mime, got, tc.want)
		}
	}
}

func TestIsSafeStoredName(t *testing.T) {
	ok := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if !IsSafeStoredName(ok) {
		t.Error("expected bare hash to be safe")
	}
	if !IsSafeStoredName(ok + ".png") {
		t.Error("expected hash.png to be safe")
	}
	if IsSafeStoredName(ok + ".%2f%2e%2e%2f.env") {
		t.Error("traversal-like stored name must be rejected")
	}
	if IsSafeStoredName("../" + ok) {
		t.Error("path prefix must be rejected")
	}
	if IsSafeStoredName(ok[:32]) {
		t.Error("short hash must be rejected")
	}
}

func TestSanitizeDownloadFilename(t *testing.T) {
	got := SanitizeDownloadFilename(`poc.%2f%2e%2e%2f.env`, "abc.png")
	if got != "abc.png" && got != "download" {
		// percent names are rejected → fallback
		if got == `poc.%2f%2e%2e%2f.env` {
			t.Fatalf("SanitizeDownloadFilename kept unsafe name %q", got)
		}
	}
	got = SanitizeDownloadFilename("normal-photo.jpg", "hash.jpg")
	if got != "normal-photo.jpg" {
		t.Errorf("got %q, want normal-photo.jpg", got)
	}
}
