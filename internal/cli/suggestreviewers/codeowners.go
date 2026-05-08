package suggestreviewers

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Codeowners is a parsed CODEOWNERS file. Rules are stored in declaration
// order; Owners returns the owners from the last matching rule, mirroring
// GitHub's "most recent match wins" semantics.
type Codeowners struct {
	Rules []Rule
}

// Rule is a single CODEOWNERS line: a path pattern plus the owners that
// own anything matching the pattern.
type Rule struct {
	Pattern string
	Owners  []string
	re      *regexp.Regexp
}

// codeownersLocations lists the standard search locations relative to a
// project root, in priority order.
var codeownersLocations = []string{
	"CODEOWNERS",
	".github/CODEOWNERS",
	"docs/CODEOWNERS",
}

// FindCodeownersFile returns the path to a CODEOWNERS file at one of the
// standard locations under root, or "" if none exist.
func FindCodeownersFile(root string) string {
	for _, rel := range codeownersLocations {
		p := filepath.Join(root, rel)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// ParseCodeownersFile reads and parses a CODEOWNERS file from disk.
func ParseCodeownersFile(path string) (*Codeowners, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open CODEOWNERS %s: %w", path, err)
	}
	defer f.Close()

	var co Codeowners
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		rule, ok := parseLine(scanner.Text())
		if !ok {
			continue
		}
		co.Rules = append(co.Rules, rule)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read CODEOWNERS %s: %w", path, err)
	}
	return &co, nil
}

// ParseCodeowners parses CODEOWNERS content from a string.
func ParseCodeowners(content string) *Codeowners {
	var co Codeowners
	for _, line := range strings.Split(content, "\n") {
		rule, ok := parseLine(line)
		if !ok {
			continue
		}
		co.Rules = append(co.Rules, rule)
	}
	return &co
}

// Owners returns the owners that match relPath. relPath must be a forward-
// slash, repo-root-relative path. Returns nil when no rule matches.
func (c *Codeowners) Owners(relPath string) []string {
	relPath = filepath.ToSlash(relPath)
	if !strings.HasPrefix(relPath, "/") {
		relPath = "/" + relPath
	}
	for i := len(c.Rules) - 1; i >= 0; i-- {
		if c.Rules[i].re.MatchString(relPath) {
			return c.Rules[i].Owners
		}
	}
	return nil
}

func parseLine(line string) (Rule, bool) {
	if i := strings.Index(line, "#"); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return Rule{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return Rule{}, false
	}
	pattern := fields[0]
	owners := make([]string, 0, len(fields)-1)
	for _, o := range fields[1:] {
		if o == "" {
			continue
		}
		owners = append(owners, o)
	}
	if len(owners) == 0 {
		return Rule{}, false
	}
	re, err := compilePattern(pattern)
	if err != nil {
		return Rule{}, false
	}
	return Rule{Pattern: pattern, Owners: owners, re: re}, true
}

// compilePattern converts a CODEOWNERS path pattern into an anchored
// regular expression matched against a leading-slash, forward-slash path.
//
// Supported features (matches GitHub's documented subset):
//   - leading "/" anchors to repo root
//   - trailing "/" matches any file under the named directory
//   - patterns without any "/" match by basename anywhere in the tree
//   - "*" matches any run of non-separator characters
//   - "**" matches any number of path segments
//   - "?" matches a single non-separator character
func normalizeGlobPattern(pattern string) (p string, dirOnly bool) {
	dirOnly = strings.HasSuffix(pattern, "/")
	p = pattern
	if dirOnly {
		p = strings.TrimSuffix(p, "/")
	}
	if !strings.Contains(p, "/") {
		p = "**/" + p
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p, dirOnly
}

func translateGlobChar(sb *strings.Builder, p string, i int) int {
	c := p[i]
	switch {
	case c == '*' && i+1 < len(p) && p[i+1] == '*':
		if i > 0 && p[i-1] == '/' && i+2 < len(p) && p[i+2] == '/' {
			s := sb.String()
			sb.Reset()
			sb.WriteString(strings.TrimSuffix(s, "/"))
			sb.WriteString("(?:/|/.*/)")
			return i + 3
		}
		sb.WriteString(".*")
		return i + 2
	case c == '*':
		sb.WriteString("[^/]*")
		return i + 1
	case c == '?':
		sb.WriteString("[^/]")
		return i + 1
	case isRegexMeta(c):
		sb.WriteByte('\\')
		sb.WriteByte(c)
		return i + 1
	default:
		sb.WriteByte(c)
		return i + 1
	}
}

func isRegexMeta(c byte) bool {
	return c == '.' || c == '+' || c == '(' || c == ')' || c == '|' || c == '^' || c == '$' ||
		c == '[' || c == ']' || c == '{' || c == '}' || c == '\\'
}

func compilePattern(pattern string) (*regexp.Regexp, error) {
	p, dirOnly := normalizeGlobPattern(pattern)
	var sb strings.Builder
	sb.WriteByte('^')
	for i := 0; i < len(p); {
		i = translateGlobChar(&sb, p, i)
	}
	if dirOnly {
		sb.WriteString("(?:/.*)?")
	}
	sb.WriteByte('$')
	return regexp.Compile(sb.String())
}
