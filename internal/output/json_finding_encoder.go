package output

import (
	"encoding/json"
	"strconv"
	"unicode/utf8"
)

// appendFindingJSON serializes one JSONFinding into dst using direct
// byte appends instead of json.Marshal+reflection. On a 87 k-finding
// payload (kotlin-corpus warm baseline) this replaces ~130 ms of
// json.Marshal calls with ~10 ms of byte concatenation — the formatter
// is otherwise CPU-bound on reflection.
//
// The output is byte-identical to json.Marshal(finding) for any
// JSONFinding value: same field order, same omitempty semantics, same
// string-escape rules. TestAppendFindingJSON_MatchesJSONMarshal pins
// that contract. The "indent" knob controls only whether we emit the
// outer-array padding; per-finding bodies are always compact (matches
// the existing json.Marshal output, which doesn't indent struct
// internals when called from a buffered Encoder).
func appendFindingJSON(dst []byte, f JSONFinding) []byte {
	dst = append(dst, '{', '"', 'f', 'i', 'l', 'e', '"', ':')
	dst = appendJSONString(dst, f.File)

	dst = append(dst, `,"line":`...)
	dst = strconv.AppendInt(dst, int64(f.Line), 10)

	dst = append(dst, `,"column":`...)
	dst = strconv.AppendInt(dst, int64(f.Column), 10)

	if f.StartByte != nil {
		dst = append(dst, `,"startByte":`...)
		dst = strconv.AppendInt(dst, int64(*f.StartByte), 10)
	}
	if f.EndByte != nil {
		dst = append(dst, `,"endByte":`...)
		dst = strconv.AppendInt(dst, int64(*f.EndByte), 10)
	}

	dst = append(dst, `,"ruleSet":`...)
	dst = appendJSONString(dst, f.RuleSet)

	dst = append(dst, `,"rule":`...)
	dst = appendJSONString(dst, f.Rule)

	dst = append(dst, `,"severity":`...)
	dst = appendJSONString(dst, f.Severity)

	dst = append(dst, `,"message":`...)
	dst = appendJSONString(dst, f.Message)

	dst = append(dst, `,"fixable":`...)
	if f.Fixable {
		dst = append(dst, 't', 'r', 'u', 'e')
	} else {
		dst = append(dst, 'f', 'a', 'l', 's', 'e')
	}

	if f.FixLevel != "" {
		dst = append(dst, `,"fixLevel":`...)
		dst = appendJSONString(dst, f.FixLevel)
	}

	if f.Confidence != 0 {
		dst = append(dst, `,"confidence":`...)
		dst = appendJSONFloat(dst, f.Confidence)
	}

	if f.Effort != "" {
		dst = append(dst, `,"effort":`...)
		dst = appendJSONString(dst, f.Effort)
	}

	if len(f.SuggestedFixes) > 0 {
		// json.Marshal on JSONSuggestedFix slices cannot fail (no channels,
		// unsupported types, or custom marshalers in the type graph), so the
		// error path is unreachable. Field ordering must match the
		// JSONFinding declaration to stay byte-identical to json.Marshal(f).
		raw, _ := json.Marshal(f.SuggestedFixes)
		dst = append(dst, `,"suggestedFixes":`...)
		dst = append(dst, raw...)
	}

	dst = append(dst, '}')
	return dst
}

// appendJSONString appends a JSON-escaped string to dst, matching the
// encoding/json package's default behavior:
//
//   - Quotes wrap the value.
//   - Backslash and double-quote get backslash-escaped.
//   - Control bytes < 0x20 use \u00XX form (matching json's default;
//     newline gets \n, tab gets \t, etc.).
//   - High-bit bytes pass through when they form valid UTF-8.
//
// HTML-safe escaping (< etc.) is INTENTIONALLY skipped: we
// configure encoding/json.Encoder without HTMLEscape in formatJSONColumnsImpl
// for the same reason — findings carry source-code snippets, not
// markup, and the extra escapes inflate output without value.
func appendJSONString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	// Hot path: scan until we find a byte that needs escaping. For
	// typical rule names, file paths, and ASCII message text every
	// byte is plain, so we can append the whole prefix as one slice.
	start := 0
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			if jsonSafe[b] {
				i++
				continue
			}
			if start < i {
				dst = append(dst, s[start:i]...)
			}
			dst = appendJSONEscapedByte(dst, b)
			i++
			start = i
			continue
		}
		// Multi-byte UTF-8 sequence — pass through unchanged. json's
		// encoder also emits raw UTF-8 by default (no \u escapes).
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	if start < len(s) {
		dst = append(dst, s[start:]...)
	}
	dst = append(dst, '"')
	return dst
}

// appendJSONEscapedByte writes a single non-safe ASCII byte using the
// shortest escape encoding/json would produce.
func appendJSONEscapedByte(dst []byte, b byte) []byte {
	switch b {
	case '"':
		return append(dst, '\\', '"')
	case '\\':
		return append(dst, '\\', '\\')
	case '\n':
		return append(dst, '\\', 'n')
	case '\r':
		return append(dst, '\\', 'r')
	case '\t':
		return append(dst, '\\', 't')
	case '\b':
		return append(dst, '\\', 'b')
	case '\f':
		return append(dst, '\\', 'f')
	}
	// Remaining control bytes (< 0x20) → \u00XX.
	const hex = "0123456789abcdef"
	return append(dst, '\\', 'u', '0', '0', hex[b>>4], hex[b&0xf])
}

// appendJSONFloat appends a float64 using the same shortest-form
// representation encoding/json uses. It mirrors the encoder in the
// standard library's encoding/json/encode.go: 'f' format for
// magnitudes in [1e-6, 1e21), 'e' format otherwise, both with -1
// precision; then trim a single leading zero from the exponent of
// negative-exponent 'e' results (`1e-07` → `1e-7`) so the output is
// byte-identical to `json.Marshal`. Notable cases: 0.75 → "0.75",
// 1 → "1", 1e-7 → "1e-7", 1e21 → "1e+21", 1e-10 → "1e-10".
func appendJSONFloat(dst []byte, v float64) []byte {
	abs := v
	if abs < 0 {
		abs = -abs
	}
	fmtByte := byte('f')
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		fmtByte = 'e'
	}
	start := len(dst)
	dst = strconv.AppendFloat(dst, v, fmtByte, -1, 64)
	if fmtByte == 'e' {
		// strconv.AppendFloat pads single-digit negative exponents to
		// two digits ("1e-07"); encoding/json strips that pad
		// ("1e-7"). Mirror the stdlib's last-four-byte fixup so our
		// output round-trips byte-identically against `json.Marshal`.
		n := len(dst)
		if n-start >= 4 && dst[n-4] == 'e' && dst[n-3] == '-' && dst[n-2] == '0' {
			dst[n-2] = dst[n-1]
			dst = dst[:n-1]
		}
	}
	return dst
}

// jsonSafe[b] is true when byte b can appear in a JSON string body
// without escaping. False for control bytes (< 0x20), `"`, and `\`.
// Indexed by ASCII byte value; high-bit bytes are handled via
// utf8.DecodeRuneInString.
var jsonSafe = func() [128]bool {
	var t [128]bool
	for i := 0x20; i < 128; i++ {
		t[i] = true
	}
	t['"'] = false
	t['\\'] = false
	return t
}()
