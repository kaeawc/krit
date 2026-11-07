package rules

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UsingHttpRule mirrors Lint's UsingHttp: an http distributionUrl in a Gradle properties file.
type UsingHttpRule struct {
	AndroidRule
}

// ProxyPasswordRule mirrors Lint's ProxyPassword: a clear-text proxy password in a properties file that
// version control may share.
type ProxyPasswordRule struct {
	AndroidRule
}

// Confidence reports a high base confidence: the check is a literal property prefix.
func (r *UsingHttpRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check is a literal property prefix.
func (r *ProxyPasswordRule) Confidence() float64 { return api.ConfidenceHigh }

// ModuleAwareNeeds declares that the rule only reads the module list, not per-module symbol data.
func (r *UsingHttpRule) ModuleAwareNeeds() ModuleAwareNeeds { return ModuleAwareNeeds{} }

// ModuleAwareNeeds declares that the rule only reads the module list, not per-module symbol data.
func (r *ProxyPasswordRule) ModuleAwareNeeds() ModuleAwareNeeds { return ModuleAwareNeeds{} }

// lintPropertyFiles lists the properties files Lint inspects: gradle.properties and local.properties of every
// project, plus the Gradle wrapper properties of the root.
func lintPropertyFiles(ctx *api.Context) []string {
	if ctx.ModuleIndex == nil || ctx.ModuleIndex.Graph == nil {
		return nil
	}
	root := ctx.ModuleIndex.Graph.RootDir
	dirs := []string{root}
	for _, mod := range ctx.ModuleIndex.Graph.Modules {
		dirs = append(dirs, mod.Dir)
	}
	var candidates []string
	for _, dir := range dirs {
		candidates = append(candidates, filepath.Join(dir, "gradle.properties"), filepath.Join(dir, "local.properties"))
	}
	candidates = append(candidates, filepath.Join(root, "gradle", "wrapper", "gradle-wrapper.properties"))
	return existingUniqueFiles(candidates)
}

type propertyLine struct {
	text   string
	offset int
	number int
}

// lintPropertyLines splits content into lines, skipping comments and lines without an `=`.
func lintPropertyLines(content string) []propertyLine {
	var out []propertyLine
	offset := 0
	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "!") && strings.Contains(line, "=") {
			out = append(out, propertyLine{text: line, offset: offset, number: i + 1})
		}
		offset += len(raw) + 1
	}
	return out
}

// lintPropertySuppressed reports whether the line before the finding is a `# noinspection <id>` comment.
func lintPropertySuppressed(content string, line propertyLine, id string) bool {
	if line.offset == 0 {
		return false
	}
	prevStart := strings.LastIndex(content[:line.offset-1], "\n") + 1
	prev := content[prevStart : line.offset-1]
	i := strings.Index(prev, "noinspection")
	return strings.HasPrefix(prev, "#") && i >= 0 && (strings.Contains(prev[i:], id) || strings.Contains(prev[i:], "all"))
}

func (r *UsingHttpRule) check(ctx *api.Context) {
	for _, path := range lintPropertyFiles(ctx) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		for _, line := range lintPropertyLines(content) {
			const prefix = "distributionUrl=http\\"
			if !strings.HasPrefix(line.text, prefix) || lintPropertySuppressed(content, line, "UsingHttp") {
				continue
			}
			ctx.Emit(lintPropertyFinding(path, line, len("distributionUrl="), len("http"),
				"Replace HTTP with HTTPS for better security; use https"+line.text[len("distributionUrl=http"):]))
		}
	}
}

func (r *ProxyPasswordRule) check(ctx *api.Context) {
	for _, path := range lintPropertyFiles(ctx) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		for _, line := range lintPropertyLines(content) {
			if !strings.HasPrefix(line.text, "systemProp.http.proxyPassword=") && !strings.HasPrefix(line.text, "systemProp.https.proxyPassword=") {
				continue
			}
			if lintGitIgnored(path) || lintPropertySuppressed(content, line, "ProxyPassword") {
				continue
			}
			valueStart := strings.Index(line.text, "=") + 1
			ctx.Emit(lintPropertyFinding(path, line, valueStart, len(line.text)-valueStart,
				"Storing passwords in clear text is risky; make sure this file is not shared or checked in via version control"))
		}
	}
}

func lintPropertyFinding(path string, line propertyLine, col, length int, msg string) scanner.Finding {
	return scanner.Finding{
		File: path, Line: line.number, Col: col + 1,
		StartByte: line.offset + col, EndByte: line.offset + col + length,
		Message: msg,
	}
}

// lintGitIgnored mirrors Lint: a .gitignore in the file's directory or an ancestor that mentions the file name.
func lintGitIgnored(path string) bool {
	name := filepath.Base(path)
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if data, err := os.ReadFile(filepath.Join(dir, ".gitignore")); err == nil && strings.Contains(string(data), name) {
			return true
		}
		if parent := filepath.Dir(dir); parent == dir {
			return false
		}
	}
}
