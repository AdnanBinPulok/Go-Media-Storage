package classes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePathRejectsEscape(t *testing.T) {
	root := t.TempDir()
	fm := &FileManager{StoragePath: root}

	// Legitimate nested path under root.
	ok, err := fm.ResolvePath("vault/abc.png")
	if err != nil {
		t.Fatalf("ResolvePath vault/abc.png: %v", err)
	}
	if !strings.HasPrefix(ok, root) {
		t.Fatalf("resolved outside root: %s", ok)
	}

	bad := []string{
		"../etc/passwd",
		"..\\etc\\passwd",
		"vault/../../etc/passwd",
		"/etc/passwd",
		"vault/%2e%2e/%2e%2e/etc/passwd",
		"vault/\x00secret",
		"%2fetc%2fpasswd",
	}
	for _, p := range bad {
		if _, err := fm.ResolvePath(p); err == nil {
			t.Errorf("ResolvePath(%q) = nil, want error", p)
		}
	}
}

func TestResolvePathStaysInRoot(t *testing.T) {
	root := t.TempDir()
	fm := &FileManager{StoragePath: root}

	vault := filepath.Join(root, "vaultfolder")
	if err := os.MkdirAll(vault, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(vault, "file.png")
	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := fm.ResolvePath("vaultfolder/file.png")
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		// On Windows Abs may differ by slash; compare cleaned.
		if filepath.Clean(got) != filepath.Clean(target) {
			t.Fatalf("got %q want %q", got, target)
		}
	}
}
