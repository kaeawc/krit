package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// pluginSpec describes how a build script can apply one plugin: by id, by the Kotlin DSL `kotlin("short")`
// helper (which stands for `org.jetbrains.kotlin.<short>`), or through a version catalog alias.
type pluginSpec struct {
	ids      []string
	idRe     *regexp.Regexp
	applyRe  *regexp.Regexp
	kotlinFn *regexp.Regexp
}

func newPluginSpec(ids []string, kotlinShort string) pluginSpec {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = regexp.QuoteMeta(id)
	}
	lit := `(["'](?:` + strings.Join(quoted, "|") + `)["'])`
	return pluginSpec{
		ids:      ids,
		idRe:     regexp.MustCompile(`\bid\s*\(?\s*` + lit + `[ \t]*\)?`),
		applyRe:  regexp.MustCompile(`\bapply\s*\(?\s*plugin\s*[:=]\s*` + lit),
		kotlinFn: regexp.MustCompile(`\bkotlin\s*\(\s*["']` + regexp.QuoteMeta(kotlinShort) + `["']\s*\)`),
	}
}

func (p pluginSpec) hasID(id string) bool {
	for _, candidate := range p.ids {
		if candidate == id {
			return true
		}
	}
	return false
}

// pluginApplication is one place a script applies a plugin.
type pluginApplication struct {
	start, end int
	// deletable is true when the whole line is just this plugin declaration.
	deletable bool
}

var (
	pluginAliasRe = regexp.MustCompile(`\balias\s*\(\s*libs\.plugins\.([A-Za-z0-9_.]+)\s*\)`)
	applyFalseRe  = regexp.MustCompile(`^\s*\)?\s*(?:version\s*(?:\(\s*)?["'][^"']*["']\s*\)?\s*)?apply\s*\(?\s*false\b`)
)

// findPluginApplications returns the places src applies the plugin. A declaration followed by `apply false` only puts
// the plugin on the classpath and is skipped. The version catalog is read only when an alias is present.
func findPluginApplications(ctx *api.Context, src *gradleSource, spec pluginSpec) []pluginApplication {
	var out []pluginApplication
	add := func(start, end, stmtStart, stmtEnd int) {
		lineStart, lineEnd := src.lineBounds(start)
		if applyFalseRe.MatchString(src.text[end:lineEnd]) {
			return
		}
		before := strings.TrimSpace(src.text[lineStart:stmtStart])
		after := strings.TrimSpace(src.text[stmtEnd:lineEnd])
		out = append(out, pluginApplication{start: start, end: end, deletable: before == "" && after == ""})
	}
	for _, m := range spec.idRe.FindAllStringSubmatchIndex(src.text, -1) {
		add(m[2], m[3], m[0], m[1])
	}
	for _, m := range spec.applyRe.FindAllStringSubmatchIndex(src.text, -1) {
		add(m[2], m[3], m[0], m[1])
	}
	for _, m := range spec.kotlinFn.FindAllStringIndex(src.text, -1) {
		add(m[0], m[1], m[0], m[1])
	}
	for _, m := range pluginAliasRe.FindAllStringSubmatchIndex(src.text, -1) {
		if spec.hasID(gradleCatalogPluginID(ctx, src.text[m[2]:m[3]])) {
			add(m[0], m[1], m[0], m[1])
		}
	}
	return out
}
