package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type customPreviewConfig struct {
	Wildcard bool
	Prefixes []string
}

func customPreviewConfigFromFields(wildcard bool, prefixes []string) customPreviewConfig {
	return customPreviewConfig{Wildcard: wildcard, Prefixes: prefixes}
}

// androidxPreviewAnnotations are the multipreview annotations that androidx.compose.ui.tooling.preview ships.
var androidxPreviewAnnotations = map[string]bool{
	"PreviewLightDark":     true,
	"PreviewFontScale":     true,
	"PreviewScreenSizes":   true,
	"PreviewDynamicColors": true,
}

func customPreviewAnnotationNameMatches(name string, cfg customPreviewConfig) bool {
	simple := flatAnnotationSimpleName(name)
	if simple == "" {
		return false
	}
	if simple == "Preview" || androidxPreviewAnnotations[simple] {
		return true
	}
	for _, prefix := range cfg.Prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if simple == prefix || simple == prefix+"Preview" || simple == prefix+"Previews" {
			return true
		}
	}
	if !cfg.Wildcard {
		return false
	}
	return strings.HasSuffix(simple, "Preview") || strings.HasSuffix(simple, "Previews") || strings.HasPrefix(simple, "Preview")
}

func textHasCustomPreviewAnnotation(text string, cfg customPreviewConfig) bool {
	for {
		at := strings.Index(text, "@")
		if at < 0 {
			return false
		}
		text = text[at+1:]
		end := 0
		for end < len(text) {
			ch := text[end]
			if (ch >= 'a' && ch <= 'z') ||
				(ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') ||
				ch == '_' || ch == '.' || ch == ':' {
				end++
				continue
			}
			break
		}
		if end > 0 && customPreviewAnnotationNameMatches(text[:end], cfg) {
			return true
		}
		text = text[end:]
	}
}

func flatHasCustomPreviewAnnotation(file *scanner.File, idx uint32, cfg customPreviewConfig) bool {
	if file == nil || idx == 0 {
		return false
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		for i := 0; i < file.FlatChildCount(mods); i++ {
			child := file.FlatChild(mods, i)
			t := file.FlatType(child)
			if t != "annotation" && t != "modifier" {
				continue
			}
			if customPreviewAnnotationNameMatches(file.FlatNodeText(child), cfg) {
				return true
			}
		}
	}
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		t := file.FlatType(child)
		if t != "annotation" && t != "modifier" {
			continue
		}
		if customPreviewAnnotationNameMatches(file.FlatNodeText(child), cfg) {
			return true
		}
	}
	return false
}

// misparsedFunctionHasPrecedingCustomPreview handles a function the grammar emitted as a prefix or infix expression,
// where the leading annotations became sibling nodes instead of modifiers.
func misparsedFunctionHasPrecedingCustomPreview(file *scanner.File, node uint32, cfg customPreviewConfig) bool {
	switch file.FlatType(node) {
	case "prefix_expression", "infix_expression", "call_expression":
	default:
		return false
	}
	text := file.FlatNodeText(node)
	if open := strings.Index(text, "{"); open >= 0 {
		text = text[:open]
	}
	if !strings.Contains(text, "fun ") {
		return false
	}
	for prev, ok := file.FlatPrevSibling(node); ok; prev, ok = file.FlatPrevSibling(prev) {
		switch file.FlatType(prev) {
		case "annotation":
			if customPreviewAnnotationNameMatches(file.FlatNodeText(prev), cfg) {
				return true
			}
		case "line_comment", "multiline_comment":
		default:
			return false
		}
	}
	return false
}
