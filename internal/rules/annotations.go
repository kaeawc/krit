package rules

import "strings"

// DefaultIgnoreAnnotated is the default set of annotations whose presence
// causes a rule to skip the annotated declaration.
//
// These are safe to include unconditionally — if a project doesn't use
// Compose or Android, these annotations won't appear and there's zero impact.
var DefaultIgnoreAnnotated = []string{
	// Compose: PascalCase function names are conventional
	"Composable",
	"androidx.compose.runtime.Composable",

	// Compose previews: only used by Android Studio, not real code
	"Preview",
	"androidx.compose.ui.tooling.preview.Preview",
	"androidx.compose.desktop.ui.tooling.preview.Preview",
}

// DefaultMagicNumberIgnoreAnnotated is the set of annotations inside which
// magic numbers are acceptable (preview functions have hardcoded dimensions, etc.)
var DefaultMagicNumberIgnoreAnnotated = []string{
	"Preview",
	"androidx.compose.ui.tooling.preview.Preview",
	"androidx.compose.desktop.ui.tooling.preview.Preview",
}

// DefaultUnusedMemberIgnoreAnnotated is the set of annotations that mark
// members as used by the framework (not dead code even if unreferenced in code).
var DefaultUnusedMemberIgnoreAnnotated = []string{
	"Preview",
	"androidx.compose.ui.tooling.preview.Preview",
	"androidx.compose.desktop.ui.tooling.preview.Preview",
	"Composable",
}

// HasIgnoredAnnotation checks if a declaration text contains any of the
// ignored annotations. Matches both simple names and fully-qualified names.
func HasIgnoredAnnotation(text string, ignoredAnnotations []string) bool {
	for _, ann := range ignoredAnnotations {
		simple := ann
		if idx := strings.LastIndex(ann, "."); idx >= 0 {
			simple = ann[idx+1:]
		}
		if strings.Contains(text, "@"+simple) {
			return true
		}
		if strings.Contains(text, "@[") {
			if annotationArrayContainsSimpleName(text, simple) {
				return true
			}
		}
	}
	return false
}

func annotationArrayContainsSimpleName(text, simple string) bool {
	i := 0
	for {
		start := strings.Index(text[i:], "@[")
		if start < 0 {
			break
		}
		start += i + 2
		end := strings.Index(text[start:], "]")
		if end < 0 {
			break
		}
		block := text[start : start+end]
		if annotationBlockContainsToken(block, simple) {
			return true
		}
		i = start + end
	}
	return false
}

func annotationBlockContainsToken(block, simple string) bool {
	j := 0
	for j < len(block) {
		for j < len(block) && (block[j] == ' ' || block[j] == '\t' || block[j] == '\n' || block[j] == ',') {
			j++
		}
		k := j
		for k < len(block) && ((block[k] >= 'a' && block[k] <= 'z') ||
			(block[k] >= 'A' && block[k] <= 'Z') ||
			(block[k] >= '0' && block[k] <= '9') ||
			block[k] == '_' || block[k] == '.') {
			k++
		}
		if k == j {
			if j < len(block) && block[j] == '(' {
				depth := 1
				j++
				for j < len(block) && depth > 0 {
					if block[j] == '(' {
						depth++
					}
					if block[j] == ')' {
						depth--
					}
					j++
				}
				continue
			}
			j++
			continue
		}
		tok := block[j:k]
		if dot := strings.LastIndex(tok, "."); dot >= 0 {
			tok = tok[dot+1:]
		}
		if tok == simple {
			return true
		}
		j = k
	}
	return false
}
