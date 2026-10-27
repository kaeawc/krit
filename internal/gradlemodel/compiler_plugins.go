package gradlemodel

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CompilerPlugins are a compilation's Kotlin compiler plugins
// (kotlinx.serialization, Compose, Parcelize, all-open, no-arg, ...): the
// plugin jars and their options in `plugin:<id>:<key>=<value>` form. Plugins
// generate declarations the frontend resolves against (`Foo.serializer()`), so
// a module that uses one only compiles cleanly under krit-fir when they are
// loaded.
//
// This is a krit-owned field of its own, never a compiler argument: `-Xplugin`
// and `-P` stay outside the KotlinSettings allowlist, and krit-fir registers
// these after its own plugin. krit-types (the Kotlin Analysis API oracle) does
// not load compiler plugins.
type CompilerPlugins struct {
	Classpath []string `json:"classpath,omitempty"`
	Options   []string `json:"options,omitempty"`
}

// IsZero reports whether no plugin would be loaded. Options without a jar are
// never applied, so only the classpath counts.
func (p CompilerPlugins) IsZero() bool { return len(p.Classpath) == 0 }

// identity is the plugin component of KotlinSettings.Identity: each jar's
// path, size and modification time (a rebuilt plugin jar changes what the
// compiler generates) and every option in order. "" for no plugins.
func (p CompilerPlugins) identity() string {
	if p.IsZero() {
		return ""
	}
	return "|plugins=" + ClasspathFingerprint(p.Classpath) + "|pluginOptions=" + strings.Join(p.Options, "\x1f")
}

// Describe names the plugin jars for verbose output.
func (p CompilerPlugins) Describe() string {
	names := make([]string, len(p.Classpath))
	for i, jar := range p.Classpath {
		names[i] = filepath.Base(jar)
	}
	return strings.Join(names, ", ")
}

var (
	pluginOptionRe = regexp.MustCompile(`^plugin:[^:=\s]+:[^=\s:]+=`)
	// kgpDefaultPluginJarRe matches what the Kotlin Gradle plugin puts on every
	// compilation's plugin classpath besides the applied plugins: Kotlin
	// scripting support and its runtime dependencies. krit-fir does not compile
	// scripts, and the embeddable scripting plugin cannot load into krit-fir's
	// compiler at all, which would take the real plugins down with it.
	kgpDefaultPluginJarRe = regexp.MustCompile(`^(kotlin-scripting-[a-z-]+|kotlin-script-runtime|kotlin-stdlib(-[a-z0-9]+)?|kotlin-reflect|kotlinx-coroutines-core(-jvm)?|annotations)-[0-9][^/]*\.jar$`)
	// jarVersionRe strips a trailing -<version>.jar to name a plugin artifact.
	jarVersionRe = regexp.MustCompile(`-[0-9][A-Za-z0-9.+_-]*\.jar$`)
)

// outputPluginOptions are plugin options that make the compiler write files
// (Compose compiler reports and metrics). krit compiles for analysis only and
// must not write into the project's build directory.
var outputPluginOptions = map[string]bool{
	"plugin:androidx.compose.compiler.plugins.kotlin:reportsDestination": true,
	"plugin:androidx.compose.compiler.plugins.kotlin:metricsDestination": true,
}

// ResolveCompilerPlugins filters a compilation's exported (or configured)
// plugin classpath and options to what krit-fir loads. The Kotlin Gradle
// plugin's own scripting support is left out (one note says so), and so are
// missing jars, malformed options and options that write build reports (each
// is in rejected with the reason). Options without any plugin jar are not
// applied.
func ResolveCompilerPlugins(classpath, options []string) (plugins CompilerPlugins, notes, rejected []string) {
	seen := map[string]bool{}
	skipped := 0
	for _, raw := range classpath {
		jar := strings.TrimSpace(raw)
		if jar == "" || seen[jar] {
			continue
		}
		seen[jar] = true
		if kgpDefaultPluginJarRe.MatchString(filepath.Base(jar)) {
			skipped++
			continue
		}
		if _, err := os.Stat(jar); err != nil {
			rejected = append(rejected, "compiler plugin "+jar+" (not found)")
			continue
		}
		plugins.Classpath = append(plugins.Classpath, jar)
	}
	if skipped > 0 {
		notes = append(notes, fmt.Sprintf("%d Kotlin scripting support entries of the compiler plugin classpath are not loaded (krit-fir does not compile scripts)", skipped))
	}
	seenOption := map[string]bool{}
	for _, raw := range options {
		option := strings.TrimSpace(raw)
		if option == "" || seenOption[option] {
			continue
		}
		seenOption[option] = true
		name, _, _ := strings.Cut(option, "=")
		switch {
		case !pluginOptionRe.MatchString(option):
			rejected = append(rejected, "compiler plugin option "+option+" (expected plugin:<id>:<key>=<value>)")
		case outputPluginOptions[name]:
			rejected = append(rejected, "compiler plugin option "+option+" (writes build reports)")
		default:
			plugins.Options = append(plugins.Options, option)
		}
	}
	if plugins.IsZero() && len(plugins.Options) > 0 {
		notes = append(notes, fmt.Sprintf("%d compiler plugin option(s) not applied: no compiler plugin jar is loaded", len(plugins.Options)))
		plugins.Options = nil
	}
	return plugins, notes, rejected
}

// WithCompilerPlugins adds plugins configured outside the Gradle model (krit.yml
// `oracle.compilerPlugins`) to s, filtered like the model's
// (ResolveCompilerPlugins) and merged with any the model supplied
// (mergeCompilerPlugins). source names the origin in notes.
func (s KotlinSettings) WithCompilerPlugins(classpath, options []string, source string) KotlinSettings {
	if len(classpath) == 0 && len(options) == 0 {
		return s
	}
	plugins, notes, rejected := ResolveCompilerPlugins(classpath, options)
	for _, note := range notes {
		s.Notes = append(s.Notes, source+": "+note)
	}
	for _, r := range rejected {
		s.Rejected = append(s.Rejected, source+": "+r)
	}
	merged, mergeNotes := mergeCompilerPlugins([]KotlinSettings{s, {Plugins: plugins}})
	s.Plugins = merged
	s.Notes = append(s.Notes, mergeNotes...)
	return s
}

// mergeCompilerPlugins combines the plugins of several source sets for the
// whole-repo compilation. Unlike the language settings, plugins are a union: a
// plugin only acts on declarations that use it (an @Serializable class, a
// @Composable function), so loading it for every source set makes the source
// sets that need it compile cleanly without changing the others. Two versions
// of one plugin artifact are never loaded together (the first is kept), and an
// option whose value differs between source sets is left at the plugin's
// default; a note says so in both cases.
func mergeCompilerPlugins(sets []KotlinSettings) (CompilerPlugins, []string) {
	var out CompilerPlugins
	var notes []string
	artifacts := map[string]string{} // artifact name -> jar kept
	for _, s := range sets {
		for _, jar := range s.Plugins.Classpath {
			artifact := jarVersionRe.ReplaceAllString(filepath.Base(jar), "")
			kept, ok := artifacts[artifact]
			switch {
			case !ok:
				artifacts[artifact] = jar
				out.Classpath = append(out.Classpath, jar)
			case kept != jar:
				notes = append(notes, fmt.Sprintf("compiler plugin %s differs across source sets; loading %s, not %s", artifact, filepath.Base(kept), filepath.Base(jar)))
			}
		}
	}
	// Per plugin option key, every source set that sets it must set the same
	// values; a multi-valued option keeps its values in order.
	type keyed struct {
		values string
		list   []string
	}
	var order []string
	byKey := map[string][]keyed{}
	for _, s := range sets {
		perSet := map[string][]string{}
		var setOrder []string
		for _, option := range s.Plugins.Options {
			key, _, _ := strings.Cut(option, "=")
			if _, ok := perSet[key]; !ok {
				setOrder = append(setOrder, key)
			}
			perSet[key] = append(perSet[key], option)
		}
		for _, key := range setOrder {
			if _, ok := byKey[key]; !ok {
				order = append(order, key)
			}
			values := strings.Join(perSet[key], "\x1f")
			known := false
			for _, k := range byKey[key] {
				known = known || k.values == values
			}
			if !known {
				byKey[key] = append(byKey[key], keyed{values, perSet[key]})
			}
		}
	}
	for _, key := range order {
		variants := byKey[key]
		if len(variants) > 1 {
			notes = append(notes, fmt.Sprintf("compiler plugin option %s differs across source sets; not applied, the plugin default is used for the whole-repo compilation", key))
			continue
		}
		out.Options = append(out.Options, variants[0].list...)
	}
	if out.IsZero() {
		out.Options = nil
	}
	return out, notes
}
