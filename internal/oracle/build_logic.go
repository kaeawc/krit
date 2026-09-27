package oracle

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

// IsBuildLogicPath recognizes buildSrc and convention-plugin included builds
// beneath the same .git boundary used by gradlemodel.Discover.
func IsBuildLogicPath(path string) bool {
	path, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	roots := gradlemodel.SettingsAncestors(path)
	if len(roots) == 0 {
		return false
	}
	root := roots[len(roots)-1]
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	for _, segment := range strings.Split(filepath.Clean(rel), string(filepath.Separator)) {
		if segment == "buildSrc" {
			return true
		}
	}
	if len(roots) < 2 {
		return false
	}
	// The nearest settings root owns the source. An arbitrary included
	// product build is not convention-plugin build logic.
	build := roots[0]
	switch filepath.Base(build) {
	case "build-logic", "buildLogic", "convention-plugins":
		return true
	}
	buildRel, err := filepath.Rel(root, build)
	if err != nil {
		return false
	}
	for _, name := range []string{"settings.gradle.kts", "settings.gradle"} {
		data, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr == nil && pluginManagementIncludesBuild(data, filepath.ToSlash(buildRel)) {
			return true
		}
	}
	return false
}

type settingsToken struct {
	kind  byte // i: identifier, s: quoted string, or punctuation itself
	value string
}

// settingsTokens is a small lexical scanner for settings scripts. It keeps
// quoted arguments while ignoring comments and all other string contents as
// code, including escaped and triple-quoted Kotlin strings.
func settingsTokens(src []byte) []settingsToken {
	var out []settingsToken
	for i := 0; i < len(src); {
		if next, ok := skipSettingsNonCode(src, i); ok {
			i = next
			continue
		}
		if token, next, ok := settingsQuoted(src, i); ok {
			out = append(out, token)
			i = next
			continue
		}
		if token, next, ok := settingsIdentifier(src, i); ok {
			out = append(out, token)
			i = next
			continue
		}
		c := src[i]
		if c == '{' || c == '}' || c == '(' || c == ')' || c == '.' {
			out = append(out, settingsToken{c, string(c)})
		}
		i++
	}
	return out
}

func skipSettingsNonCode(src []byte, i int) (int, bool) {
	if src[i] == '$' && i+1 < len(src) && src[i+1] == '/' {
		for j := i + 2; j+1 < len(src); j++ {
			if src[j] == '/' && src[j+1] == '$' {
				return j + 2, true
			}
		}
		return len(src), true
	}
	if src[i] != '/' || i+1 >= len(src) {
		return i, false
	}
	switch src[i+1] {
	case '/':
		j := i + 2
		for j < len(src) && src[j] != '\n' {
			j++
		}
		return j, true
	case '*':
		return skipSettingsBlockComment(src, i+2), true
	}
	// Groovy slashy string. A complete slash-delimited token is required.
	if src[i+1] == ' ' || src[i+1] == '\n' || src[i+1] == '\t' {
		return i, false
	}
	for j := i + 1; j < len(src) && src[j] != '\n'; j++ {
		if src[j] == '\\' {
			j++
			continue
		}
		if src[j] == '/' {
			return j + 1, true
		}
	}
	return i, false
}

func skipSettingsBlockComment(src []byte, i int) int {
	depth := 1
	for i+1 < len(src) && depth > 0 {
		if src[i] == '/' && src[i+1] == '*' {
			depth++
			i += 2
			continue
		}
		if src[i] == '*' && src[i+1] == '/' {
			depth--
			i += 2
			continue
		}
		i++
	}
	if depth > 0 {
		return len(src)
	}
	return i
}

func settingsQuoted(src []byte, i int) (settingsToken, int, bool) {
	quote := src[i]
	if quote != '\'' && quote != '"' {
		return settingsToken{}, i, false
	}
	triple := quote == '"' && i+2 < len(src) && src[i+1] == quote && src[i+2] == quote
	start := i + 1
	if triple {
		start = i + 3
	}
	for j := start; j < len(src); j++ {
		if !triple && src[j] == '\\' {
			j++
			continue
		}
		if triple && j+2 < len(src) && src[j] == quote && src[j+1] == quote && src[j+2] == quote {
			return settingsToken{'s', string(src[start:j])}, j + 3, true
		}
		if !triple && src[j] == quote {
			return settingsToken{'s', string(src[start:j])}, j + 1, true
		}
	}
	return settingsToken{'s', string(src[start:])}, len(src), true
}

func settingsIdentifier(src []byte, i int) (settingsToken, int, bool) {
	if !settingsIdentStart(src[i]) {
		return settingsToken{}, i, false
	}
	j := i + 1
	for j < len(src) && (settingsIdentStart(src[j]) || src[j] >= '0' && src[j] <= '9') {
		j++
	}
	return settingsToken{'i', string(src[i:j])}, j, true
}

func settingsIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func pluginManagementIncludesBuild(src []byte, rel string) bool {
	tokens := settingsTokens(src)
	var scopes []bool
	for i, token := range tokens {
		switch token.kind {
		case '{':
			scopes = append(scopes, (len(scopes) > 0 && scopes[len(scopes)-1]) || settingsPluginBlock(tokens, i))
		case '}':
			if len(scopes) > 0 {
				scopes = scopes[:len(scopes)-1]
			}
		case 'i':
			if len(scopes) == 0 || !scopes[len(scopes)-1] || !settingsIncludeCall(tokens, i, rel) {
				continue
			}
			return true
		}
	}
	return false
}

func settingsPluginBlock(tokens []settingsToken, i int) bool {
	return i > 0 && tokens[i-1].kind == 'i' && tokens[i-1].value == "pluginManagement" && (i < 2 || tokens[i-2].kind != '.')
}

func settingsIncludeCall(tokens []settingsToken, i int, rel string) bool {
	if tokens[i].value != "includeBuild" || i > 0 && tokens[i-1].kind == '.' {
		return false
	}
	j := i + 1
	if j < len(tokens) && tokens[j].kind == '(' {
		j++
	}
	return j < len(tokens) && tokens[j].kind == 's' && filepath.Clean(tokens[j].value) == filepath.Clean(rel)
}

// FilterFIRSourceDirs removes build-logic roots from a FIR compilation.
func FilterFIRSourceDirs(dirs []string) []string {
	var out []string
	for _, dir := range dirs {
		if !IsBuildLogicPath(dir) {
			out = append(out, dir)
		}
	}
	return out
}
