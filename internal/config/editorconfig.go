package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// EditorConfig holds parsed .editorconfig properties relevant to krit.
// Only reads standard properties that affect analysis rules.
// Does NOT read ktlint_* or ktfmt_* properties. From the ij_* namespace,
// only ij_kotlin_indent_size is consulted, as a fallback for indent_size
// when projects configure indent width via the IntelliJ key alone.
type EditorConfig struct {
	MaxLineLength          int    // max_line_length (0 = not set, -1 = off)
	IndentSize             int    // indent_size (0 = not set)
	TabWidth               int    // tab_width (0 = not set)
	IndentStyle            string // "space" or "tab" or ""
	Charset                string // charset or ""
	InsertFinalNewline     *bool  // insert_final_newline (nil = not set)
	TrimTrailingWhitespace *bool  // trim_trailing_whitespace (nil = not set)
}

// LoadEditorConfig walks up the directory tree from startPath looking for
// .editorconfig files. Merges properties from all found files (closest wins).
// Stops at a file with root = true.
func LoadEditorConfig(startPath string) *EditorConfig {
	ec := &EditorConfig{}

	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return ec
	}

	// If startPath is a file, use its directory
	info, err := os.Stat(absPath)
	if err != nil {
		return ec
	}
	dir := absPath
	if !info.IsDir() {
		dir = filepath.Dir(absPath)
	}

	// Walk up collecting .editorconfig files (closest first)
	var configs []map[string]string
	for {
		ecPath := filepath.Join(dir, ".editorconfig")
		if props, isRoot := parseEditorConfig(ecPath); props != nil {
			configs = append(configs, props)
			if isRoot {
				break
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	// Apply in reverse order (farthest first, closest overrides)
	for i := len(configs) - 1; i >= 0; i-- {
		applyProps(ec, configs[i])
	}

	return ec
}

// parseEditorConfig reads a .editorconfig file and returns properties
// matching [*.{kt,kts}] or [*] sections. Returns (nil, false) if file doesn't exist.
func parseEditorConfig(path string) (props map[string]string, isRoot bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	props = make(map[string]string)
	inMatchingSection := false
	inGlobalSection := true // before any section header

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Check for root = true (before any section)
		if inGlobalSection && strings.HasPrefix(strings.ToLower(line), "root") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(strings.ToLower(parts[1])) == "true" {
				isRoot = true
			}
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inGlobalSection = false
			section := line[1 : len(line)-1]
			inMatchingSection = matchesKotlin(section)
			continue
		}

		// Property
		if inMatchingSection {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(strings.ToLower(parts[0]))
				value := strings.TrimSpace(parts[1])
				// Only read standard properties — skip ktlint_*, ktfmt_*, and
				// most ij_* keys. ij_kotlin_indent_size is allowed through as
				// a fallback for the standard indent_size key.
				if !strings.HasPrefix(key, "ktlint_") &&
					!strings.HasPrefix(key, "ktfmt_") &&
					(!strings.HasPrefix(key, "ij_") || key == "ij_kotlin_indent_size") {
					props[key] = value
				}
			}
		}
	}

	if len(props) == 0 && !isRoot {
		return nil, false
	}
	return props, isRoot
}

// matchesKotlin reports whether an editorconfig section glob matches Kotlin
// source files (.kt or .kts). Only true Kotlin-applicable globs return true;
// substring lookalikes such as `*.kt.tmpl`, `*.ktx`, `*.ktm`, or `hot_keys.cfg`
// must not apply Kotlin settings.
//
// The .editorconfig spec uses fnmatch-style globs with optional brace
// alternation. We support the common forms used in practice:
//   - `*` (universal)
//   - `*.kt`, `*.kts`
//   - compound extension lists: `*.{kt,kts}`, `*.{java,kt,kts}`
//   - top-level brace alternation: `{*.kt,*.kts}`, `{*.kt,Makefile}`
//
// Patterns are anchored at the end after brace expansion: only branches that
// actually terminate in `.kt` or `.kts` count as Kotlin matches.
func matchesKotlin(section string) bool {
	section = strings.TrimSpace(section)
	if section == "" {
		return false
	}
	if section == "*" {
		return true
	}
	for _, branch := range expandBraces(section) {
		if kotlinBranchMatches(branch) {
			return true
		}
	}
	return false
}

// kotlinBranchMatches reports whether a single (brace-free) glob branch
// matches Kotlin source files. The pattern must end with `.kt` or `.kts` —
// substring matches like `*.kt.tmpl` or `*.ktx` are rejected.
func kotlinBranchMatches(branch string) bool {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false
	}
	lower := strings.ToLower(branch)
	return strings.HasSuffix(lower, ".kt") || strings.HasSuffix(lower, ".kts")
}

// expandBraces performs a single level of brace alternation expansion on a
// glob pattern. `*.{kt,kts}` expands to `*.kt`, `*.kts`; `{*.kt,*.kts}`
// expands to `*.kt`, `*.kts`. Patterns without braces are returned as-is.
// Nested or malformed braces fall back to returning the original string.
func expandBraces(pattern string) []string {
	openIdx := strings.IndexByte(pattern, '{')
	if openIdx < 0 {
		return []string{pattern}
	}
	closeIdx := strings.IndexByte(pattern[openIdx:], '}')
	if closeIdx < 0 {
		return []string{pattern}
	}
	closeIdx += openIdx

	prefix := pattern[:openIdx]
	suffix := pattern[closeIdx+1:]
	inner := pattern[openIdx+1 : closeIdx]

	var out []string
	for _, alt := range strings.Split(inner, ",") {
		alt = strings.TrimSpace(alt)
		expanded := prefix + alt + suffix
		// Allow one additional pass for patterns like `*.{kt,kts}` nested
		// inside an outer `{...,...}` block.
		out = append(out, expandBraces(expanded)...)
	}
	return out
}

func applyProps(ec *EditorConfig, props map[string]string) {
	if v, ok := props["max_line_length"]; ok {
		if strings.ToLower(v) == "off" {
			ec.MaxLineLength = -1
		} else if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ec.MaxLineLength = n
		}
	}

	// Resolve tab_width first so indent_size = tab in the same section sees
	// the section's own tab_width rather than stale state from a previous
	// merge pass or a zero default. map iteration order is randomized, so
	// the previous ordering by chance was unreliable.
	if v, ok := props["tab_width"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ec.TabWidth = n
		}
	}

	if v, ok := props["indent_size"]; ok {
		if strings.ToLower(v) == "tab" {
			// indent_size = tab means use tab_width
			if ec.TabWidth > 0 {
				ec.IndentSize = ec.TabWidth
			}
		} else if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ec.IndentSize = n
		}
	} else if v, ok := props["ij_kotlin_indent_size"]; ok {
		// Fallback: projects that only set the IntelliJ-style key still
		// get a coherent indent width for indent-touching fixes (NoTabs).
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ec.IndentSize = n
		}
	}

	if v, ok := props["indent_style"]; ok {
		ec.IndentStyle = strings.ToLower(v)
	}

	if v, ok := props["charset"]; ok {
		ec.Charset = strings.ToLower(v)
	}

	if v, ok := props["insert_final_newline"]; ok {
		b := strings.ToLower(v) == "true"
		ec.InsertFinalNewline = &b
	}

	if v, ok := props["trim_trailing_whitespace"]; ok {
		b := strings.ToLower(v) == "true"
		ec.TrimTrailingWhitespace = &b
	}
}

// ApplyEditorConfigToRules updates rule configuration based on .editorconfig values.
// Called after YAML config is loaded — .editorconfig values override YAML for
// the properties it covers (matching ktfmt's behavior).
func (ec *EditorConfig) ApplyToConfig(cfg *Config) {
	if ec == nil || cfg == nil {
		return
	}

	// max_line_length → MaxLineLength rule
	if ec.MaxLineLength > 0 {
		cfg.Set("style", "MaxLineLength", "maxLineLength", ec.MaxLineLength)
	} else if ec.MaxLineLength == -1 {
		// "off" → disable the rule
		cfg.Set("style", "MaxLineLength", "active", false)
	}

	// insert_final_newline = false → disable NewLineAtEndOfFile
	if ec.InsertFinalNewline != nil && !*ec.InsertFinalNewline {
		cfg.Set("style", "NewLineAtEndOfFile", "active", false)
	}

	// trim_trailing_whitespace = false → disable TrailingWhitespace
	if ec.TrimTrailingWhitespace != nil && !*ec.TrimTrailingWhitespace {
		cfg.Set("style", "TrailingWhitespace", "active", false)
	}

	// indent_style = tab → disable NoTabs rule
	if ec.IndentStyle == "tab" {
		cfg.Set("style", "NoTabs", "active", false)
	}

	// indent_size / tab_width → NoTabs fix replacement width
	indentWidth := ec.IndentSize
	if indentWidth <= 0 {
		indentWidth = ec.TabWidth
	}
	if indentWidth > 0 {
		cfg.Set("style", "NoTabs", "indentSize", indentWidth)
	}
}
