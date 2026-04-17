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
		// Extract simple name from FQN
		simple := ann
		if idx := strings.LastIndex(ann, "."); idx >= 0 {
			simple = ann[idx+1:]
		}
		// Match plain `@SimpleName` form.
		if strings.Contains(text, "@"+simple) {
			return true
		}
		// Match Kotlin's annotation-array syntax:
		//   @[Composable Preview(showBackground = true)]
		//   @[Composable Preview]
		// where `@[ ... ]` groups multiple annotations. The names
		// inside aren't prefixed with `@` individually.
		if strings.Contains(text, "@[") {
			// Find each `@[...]` block and check whether our simple
			// name appears as a whole-word token inside.
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
				// Tokenize on whitespace / `(` to ignore argument lists.
				j := 0
				for j < len(block) {
					// Skip whitespace.
					for j < len(block) && (block[j] == ' ' || block[j] == '\t' || block[j] == '\n' || block[j] == ',') {
						j++
					}
					// Read identifier.
					k := j
					for k < len(block) && ((block[k] >= 'a' && block[k] <= 'z') ||
						(block[k] >= 'A' && block[k] <= 'Z') ||
						(block[k] >= '0' && block[k] <= '9') ||
						block[k] == '_' || block[k] == '.') {
						k++
					}
					if k == j {
						// Advance past any non-identifier char (likely `(`).
						// Skip a balanced arg list if present.
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
					// Strip FQN prefix.
					if dot := strings.LastIndex(tok, "."); dot >= 0 {
						tok = tok[dot+1:]
					}
					if tok == simple {
						return true
					}
					j = k
				}
				i = start + end
			}
		}
	}
	return false
}
