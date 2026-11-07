package rules

import (
	"bytes"
	"regexp"
	"strings"
)

// viewIDBindingName camel-cases a layout id the way view binding names its property (`export_item_button` to `exportItemButton`).
func viewIDBindingName(id string) string {
	parts := strings.Split(id, "_")
	var b strings.Builder
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			b.WriteString(part)
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

var (
	// `name.contentDescription = ...` and `name.setContentDescription(...)`, with an optional safe call or assertion.
	describedDirectPattern = regexp.MustCompile(`(\w+)\s*(?:\?|!!)?\s*\.\s*(?:contentDescription\s*=[^=]|setContentDescription\s*\()`)
	// `name?.let {`, `name.apply {` and similar scope calls whose block may describe the receiver.
	describedScopePattern = regexp.MustCompile(`(\w+)\s*(?:\?|!!)?\s*\.\s*(?:let|apply|run|also)\s*\{`)
)

// ComputeDescribedViewNames returns the names of views that source files give a content description at runtime:
// a `contentDescription` assignment or a `setContentDescription` call.
// Receivers inside `x?.let { it... }` scope blocks resolve to `x`.
func ComputeDescribedViewNames(sources [][]byte) map[string]bool {
	names := map[string]bool{}
	for _, src := range sources {
		if !bytes.Contains(src, []byte("ontentDescription")) {
			continue
		}
		collectDescribedNames(string(src), "", names)
		for _, m := range describedScopePattern.FindAllStringSubmatchIndex(string(src), -1) {
			block := describedBlockAfter(string(src), m[1]-1)
			collectDescribedNames(block, string(src)[m[2]:m[3]], names)
		}
	}
	return names
}

// collectDescribedNames adds the receivers in text that get a description. scopeName replaces the receivers `it` and `this`.
func collectDescribedNames(text, scopeName string, names map[string]bool) {
	add := func(name string) {
		if name == "it" || name == "this" {
			name = scopeName
		}
		if name != "" {
			names[name] = true
		}
	}
	for _, m := range describedDirectPattern.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
}

// describedBlockAfter returns the brace-balanced block text that starts at the `{` at open.
func describedBlockAfter(text string, open int) string {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[open : i+1]
			}
		}
	}
	return text[open:]
}
