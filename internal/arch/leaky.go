package arch

import (
	"regexp"
	"strings"
)

// LeakyClass represents a public class that appears to be a thin wrapper.
type LeakyClass struct {
	ClassName         string
	WrappedType       string
	Line              int
	DelegationRatio   float64 // 0.0-1.0, fraction of methods that delegate
	TotalMethods      int
	DelegatingMethods int
}

var (
	// Matches: class Foo(  or  open class Foo(  but not abstract/private/internal/protected class
	classDeclRe = regexp.MustCompile(`^\s*(?:open\s+)?class\s+(\w+)\s*\(`)
	// Matches abstract class
	abstractClassRe = regexp.MustCompile(`^\s*abstract\s+class\s+`)
	// Matches private/internal/protected class
	nonPublicClassRe = regexp.MustCompile(`^\s*(?:private|internal|protected)\s+(?:open\s+)?class\s+`)
	// Matches a single private val param: private val name: Type
	singlePrivateValRe = regexp.MustCompile(`^\s*(?:private\s+)?val\s+(\w+)\s*:\s*(\w+)`)
	// Matches a public method (fun keyword, no private/internal/protected)
	publicMethodRe = regexp.MustCompile(`^\s*(?:override\s+)?fun\s+(\w+)\s*\(`)
	// Matches private/internal/protected fun
	nonPublicMethodRe = regexp.MustCompile(`^\s*(?:private|internal|protected)\s+(?:override\s+)?fun\s+`)
	// Matches single-expression delegation: fun name(...) = delegate.name(...)
	// or fun name(...): Type = delegate.name(...)
	delegationRe = regexp.MustCompile(`^\s*(?:override\s+)?fun\s+\w+\s*\([^)]*\)\s*(?::\s*\S+)?\s*=\s*(\w+)\.\w+\(`)
)

// DetectLeakyAbstractions scans a file for public classes that are thin
// wrappers delegating to a single private/internal field.
//
// Heuristic:
// 1. Find public class declarations with a single constructor parameter
// 2. Check if the parameter is stored as a private/internal property
// 3. Count public methods that are single-expression delegations to that property
// 4. If delegation ratio > threshold, flag as leaky
func DetectLeakyAbstractions(lines []string, threshold float64) []LeakyClass {
	var results []LeakyClass

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Skip abstract classes
		if abstractClassRe.MatchString(line) {
			continue
		}
		// Skip non-public classes
		if nonPublicClassRe.MatchString(line) {
			continue
		}

		classMatch := classDeclRe.FindStringSubmatch(line)
		if classMatch == nil {
			continue
		}

		className := classMatch[1]
		classLine := i + 1 // 1-based

		// Extract constructor content — may span the same line or next lines
		ctorContent := extractConstructorParams(lines, i)
		if ctorContent == "" {
			continue
		}

		// Parse constructor params — we only care about single-param wrappers
		fieldName, fieldType := parseSinglePrivateVal(ctorContent)
		if fieldName == "" {
			continue
		}

		// Scan methods in this class body
		totalMethods, delegatingMethods := countMethods(lines, i, fieldName)
		if totalMethods == 0 {
			continue
		}

		ratio := float64(delegatingMethods) / float64(totalMethods)
		if ratio > threshold {
			results = append(results, LeakyClass{
				ClassName:         className,
				WrappedType:       fieldType,
				Line:              classLine,
				DelegationRatio:   ratio,
				TotalMethods:      totalMethods,
				DelegatingMethods: delegatingMethods,
			})
		}
	}

	return results
}

// extractConstructorParams gets the text between the opening ( and closing ) of a class constructor.
func extractConstructorParams(lines []string, startIdx int) string {
	var buf strings.Builder
	depth := 0
	started := false

	for i := startIdx; i < len(lines); i++ {
		for _, ch := range lines[i] {
			if ch == '(' {
				if !started {
					started = true
					depth = 1
					continue
				}
				depth++
			} else if ch == ')' {
				depth--
				if depth == 0 {
					return buf.String()
				}
			}
			if started && depth > 0 {
				buf.WriteRune(ch)
			}
		}
		if started {
			buf.WriteByte(' ')
		}
		// Don't look more than a few lines ahead for the constructor
		if i > startIdx+5 {
			break
		}
	}
	return ""
}

// parseSinglePrivateVal checks if the constructor content has exactly one parameter
// that is a private val (or just val, which we treat as a stored field).
func parseSinglePrivateVal(content string) (name, typeName string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", ""
	}

	// Split by comma to check param count
	params := splitParams(content)
	if len(params) != 1 {
		return "", ""
	}

	param := strings.TrimSpace(params[0])

	// Must contain "val" to be a stored property
	if !strings.Contains(param, "val ") {
		return "", ""
	}

	m := singlePrivateValRe.FindStringSubmatch(param)
	if m == nil {
		return "", ""
	}

	return m[1], m[2]
}

// splitParams splits constructor params by commas, respecting nested parens/generics.
func splitParams(s string) []string {
	var parts []string
	depth := 0
	start := 0

	for i, ch := range s {
		switch ch {
		case '(', '<':
			depth++
		case ')', '>':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	part := strings.TrimSpace(s[start:])
	if part != "" {
		parts = append(parts, part)
	}
	return parts
}

// isBlockBodyDelegation checks whether the method starting at lineIdx has a
// block body that contains only a single call to fieldName.<method>(...).
// Recognizes forms like:
//   fun save(user: User) {
//       impl.save(user)
//   }
func isBlockBodyDelegation(lines []string, lineIdx int, fieldName string) bool {
	// Require the method header to end with '{' on the same or following line
	openIdx := -1
	for i := lineIdx; i < len(lines) && i < lineIdx+3; i++ {
		if strings.Contains(lines[i], "{") {
			openIdx = i
			break
		}
		if strings.Contains(lines[i], "=") {
			return false // expression body, handled elsewhere
		}
	}
	if openIdx == -1 {
		return false
	}

	depth := 0
	statements := 0
	delegates := false
	prefix := fieldName + "."

	for i := openIdx; i < len(lines); i++ {
		line := lines[i]
		for _, ch := range line {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
			}
		}

		if i > openIdx {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "//") {
				// skip
			} else if depth <= 0 {
				// closing brace line
			} else {
				statements++
				if idx := strings.Index(trimmed, prefix); idx >= 0 && strings.Contains(trimmed[idx+len(prefix):], "(") {
					delegates = true
				}
			}
		}

		if depth <= 0 && i > openIdx {
			break
		}
	}

	return statements == 1 && delegates
}

// countMethods counts total public methods and delegating methods in a class body.
func countMethods(lines []string, classIdx int, fieldName string) (total, delegating int) {
	// Find the opening brace of the class body
	braceIdx := -1
	for i := classIdx; i < len(lines) && i < classIdx+10; i++ {
		if strings.Contains(lines[i], "{") {
			braceIdx = i
			break
		}
	}
	if braceIdx == -1 {
		return 0, 0
	}

	// Track brace depth to know when we exit the class
	depth := 0
	for i := braceIdx; i < len(lines); i++ {
		for _, ch := range lines[i] {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
			}
		}

		// We're inside the class body (depth >= 1 after counting the class brace)
		if depth <= 0 && i > braceIdx {
			break
		}

		// Only count methods at class level (depth == 1)
		if nonPublicMethodRe.MatchString(lines[i]) {
			continue
		}

		if publicMethodRe.MatchString(lines[i]) {
			total++

			// Check if this is a single-expression delegation to our field
			if m := delegationRe.FindStringSubmatch(lines[i]); m != nil && m[1] == fieldName {
				delegating++
			} else if isBlockBodyDelegation(lines, i, fieldName) {
				delegating++
			}
		}
	}
	return total, delegating
}
