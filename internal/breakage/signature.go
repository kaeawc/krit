package breakage

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// Normalize collapses transient details out of a failure message so
// the same underlying breakage produces the same signature across
// reruns. The goal is not a perfect canonicalisation — it's "stable
// enough to join events from different runs to the same bucket."
//
// Transformations:
//   - strip ANSI escape sequences;
//   - collapse hex addresses / pointers (0x...) to <addr>;
//   - collapse any decimal run of >=3 digits to <n> (file line numbers,
//     durations, durations-in-ms, retries);
//   - collapse repeated whitespace to a single space;
//   - lowercase and trim.
//
// Callers that want to keep numeric detail (e.g. a finding count) should
// pass the structured field separately rather than baking it into the
// raw signature string.
var (
	ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	hexRe  = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	numRe  = regexp.MustCompile(`\d{3,}`)
	wsRe   = regexp.MustCompile(`\s+`)
)

func Normalize(raw string) string {
	if raw == "" {
		return ""
	}
	s := ansiRe.ReplaceAllString(raw, "")
	s = hexRe.ReplaceAllString(s, "<addr>")
	s = numRe.ReplaceAllString(s, "<n>")
	s = wsRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(strings.ToLower(s))
}

// HashID computes a stable 16-char hex ID for an event from its
// normalised signature, failure kind, commit sha, and source. The ID
// is content-derived so re-ingesting the same event from the same
// source is idempotent — store-level dedupe leans on it.
func HashID(kind, signature, commitSHA, source string) string {
	h := sha256.New()
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write([]byte(signature))
	h.Write([]byte{0})
	h.Write([]byte(commitSHA))
	h.Write([]byte{0})
	h.Write([]byte(source))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
