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

func customPreviewAnnotationNameMatches(name string, cfg customPreviewConfig) bool {
	simple := flatAnnotationSimpleName(name)
	if simple == "" {
		return false
	}
	if simple == "Preview" {
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
	return strings.HasSuffix(simple, "Preview") || strings.HasSuffix(simple, "Previews")
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
