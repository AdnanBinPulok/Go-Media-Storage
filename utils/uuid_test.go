package utils

import "testing"

func TestNormalizeUUID(t *testing.T) {
	const canonical = "550e8400-e29b-41d4-a716-446655440000"

	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"canonical", canonical, canonical, true},
		{"uppercase", "550E8400-E29B-41D4-A716-446655440000", canonical, true},
		{"mixed case", "550e8400-E29B-41d4-A716-446655440000", canonical, true},
		{"32 hex", "550e8400e29b41d4a716446655440000", canonical, true},
		{"uppercase 32 hex", "550E8400E29B41D4A716446655440000", canonical, true},
		{"braced", "{550e8400-e29b-41d4-a716-446655440000}", canonical, true},
		{"urn", "urn:uuid:550e8400-e29b-41d4-a716-446655440000", canonical, true},
		{"urn uppercase", "URN:UUID:550E8400-E29B-41D4-A716-446655440000", canonical, true},
		{"leading space", "  " + canonical + "  ", canonical, true},
		{"no hyphens no case", "6ba7b8109dad11d180b400c04fd430c8", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", true},

		{"empty", "", "", false},
		{"not a uuid", "user_42", "", false},
		{"too short", "550e8400-e29b-41d4-a716-44665544000", "", false},
		{"too long", canonical + "0", "", false},
		{"non hex", "550e8400-e29b-41d4-a716-44665544000g", "", false},
		{"sql injection", "1' OR '1'='1", "", false},
		{"path traversal", "../../etc/passwd", "", false},
		{"spaces inside", "550e8400 e29b 41d4 a716 446655440000", "", false},
	}

	for _, tc := range cases {
		got, ok := NormalizeUUID(tc.in)
		if ok != tc.ok {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.ok)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNormalizeUUIDIsIdempotent(t *testing.T) {
	const canonical = "550e8400-e29b-41d4-a716-446655440000"

	once, ok := NormalizeUUID(canonical)
	if !ok {
		t.Fatal("canonical form rejected")
	}
	twice, ok := NormalizeUUID(once)
	if !ok {
		t.Fatal("normalized form rejected")
	}
	if once != twice {
		t.Errorf("not idempotent: %q then %q", once, twice)
	}
}

// Hyphens are ignored entirely while parsing, so odd placements still reduce to
// 32 hex characters. That is intentional: the input is a UUID either way.
func TestNormalizeUUIDHyphenPlacement(t *testing.T) {
	got, ok := NormalizeUUID("550e8400e29b-41d4a716-446655440000")
	if !ok {
		t.Fatal("hyphen placement with 32 hex chars rejected")
	}
	if got != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("got %q", got)
	}
}
