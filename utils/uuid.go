package utils

import "strings"

// uuidHexLen is the length of a UUID with its hyphens removed.
const uuidHexLen = 32

// NormalizeUUID accepts every textual UUID form and returns the canonical
// lowercase 8-4-4-4-12 spelling.
//
// Accepted: canonical, uppercase canonical, bare 32 hex characters, braced
// "{...}", and "urn:uuid:" prefixed. Everything else is rejected.
//
// Normalizing matters for two reasons: the avatar is a pure function of the id,
// so "550E..." and "550e..." must not render differently, and they must share
// one cache entry.
func NormalizeUUID(raw string) (string, bool) {
	s := strings.TrimSpace(raw)

	if len(s) >= 9 && strings.EqualFold(s[:9], "urn:uuid:") {
		s = s[9:]
	}

	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")

	hex := make([]byte, 0, uuidHexLen)
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '-':
			// Layout is rebuilt below, so stray hyphens are only rejected by the
			// final length check.
		case ch >= '0' && ch <= '9':
			hex = append(hex, ch)
		case ch >= 'a' && ch <= 'f':
			hex = append(hex, ch)
		case ch >= 'A' && ch <= 'F':
			hex = append(hex, ch+('a'-'A'))
		default:
			return "", false
		}
	}

	if len(hex) != uuidHexLen {
		return "", false
	}

	out := make([]byte, 0, uuidHexLen+4)
	out = append(out, hex[0:8]...)
	out = append(out, '-')
	out = append(out, hex[8:12]...)
	out = append(out, '-')
	out = append(out, hex[12:16]...)
	out = append(out, '-')
	out = append(out, hex[16:20]...)
	out = append(out, '-')
	out = append(out, hex[20:32]...)
	return string(out), true
}
