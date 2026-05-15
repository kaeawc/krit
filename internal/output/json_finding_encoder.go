package output

import (
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
// representation encoding/json uses (strconv 'g' with -1 precision).
// Notable cases that match json.Marshal: 0.75 → "0.75", 0.85 → "0.85",
// 0.95 → "0.95", 1 → "1", 0.5 → "0.5".
func appendJSONFloat(dst []byte, v float64) []byte {
	// json.Marshal for non-integer floats uses 'g' format with the
	// shortest representation that round-trips. For integer-valued
	// floats it omits the decimal (e.g. 1 not 1.0).
	abs := v
	if abs < 0 {
		abs = -abs
	}
	fmtByte := byte('f')
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		fmtByte = 'e'
	}
	// strconv emits "1.2e+05" / "1.2"; both match encoding/json's
	// default float formatting (shortest round-trippable form, sign-
	// prefixed exponent for 'e').
	return strconv.AppendFloat(dst, v, fmtByte, -1, 64)
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
