package gradlemodel

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
)

// KotlinOptions are the Kotlin compiler settings krit-gradle-plugin exports for
// one source set (the `kotlin` object of a schema 2 model). Empty values mean
// "not configured on the compilation"; the effective values are computed by
// ResolveKotlinSettings, never by the exporter.
type KotlinOptions struct {
	LanguageVersion  string   `json:"languageVersion"`
	APIVersion       string   `json:"apiVersion"`
	OptIn            []string `json:"optIn"`
	FreeCompilerArgs []string `json:"freeCompilerArgs"`
	ProgressiveMode  bool     `json:"progressiveMode"`
	// PluginVersion is the Kotlin Gradle plugin version, used only to derive the
	// language version Gradle itself would have used when none is configured.
	PluginVersion string `json:"pluginVersion"`
	// CompilerPluginClasspath and CompilerPluginOptions are the compile task's
	// compiler plugin jars and `plugin:<id>:<key>=<value>` options (see
	// CompilerPlugins). Absent in a model exported before they existed.
	CompilerPluginClasspath []string `json:"compilerPluginClasspath,omitempty"`
	CompilerPluginOptions   []string `json:"compilerPluginOptions,omitempty"`
}

// KotlinSettings is the effective, allowlist-filtered set of Kotlin compiler
// settings that change how source is analyzed. It is what krit sends to the
// JVM helpers and folds into cache identities.
//
// The zero value means "no project settings known": the embedded compiler's
// own defaults apply. That is the documented behavior for a missing,
// schema 1, or incomplete Gradle model, and it is never silent: Notes says why.
type KotlinSettings struct {
	LanguageVersion string   `json:"languageVersion,omitempty"`
	APIVersion      string   `json:"apiVersion,omitempty"`
	OptIns          []string `json:"optIns,omitempty"`
	Progressive     bool     `json:"progressive,omitempty"`
	// Flags are the allowlisted -X / -XXLanguage: flags in application order
	// (a later flag wins over an earlier contradictory one).
	Flags []string `json:"flags,omitempty"`
	// Plugins are the compiler plugins krit-fir loads. They are not compiler
	// arguments: CompilerArgs never renders them, and they reach krit-fir in
	// fields of their own.
	Plugins CompilerPlugins `json:"plugins,omitzero"`

	// Notes explain defaulting decisions and Rejected lists every compiler
	// argument that was not applied, with the reason. Neither is part of the
	// identity: they describe why the settings are what they are, and are
	// surfaced in verbose output and perf attributes.
	Notes    []string `json:"notes,omitempty"`
	Rejected []string `json:"rejected,omitempty"`
}

// IsZero reports whether no setting would reach the compiler.
func (s KotlinSettings) IsZero() bool {
	return s.LanguageVersion == "" && s.APIVersion == "" && len(s.OptIns) == 0 && !s.Progressive && len(s.Flags) == 0 &&
		s.Plugins.IsZero()
}

// kotlinSettingsIdentityVersion changes when the identity encoding or the
// meaning of an applied setting changes, so old cache entries are not reused.
const kotlinSettingsIdentityVersion = 1

// Identity is a canonical string for the semantic settings, "" for the zero
// value (so projects without a model keep their existing cache keys). Notes
// and Rejected are excluded; opt-ins are sorted; flags keep application order.
// Compiler plugins add their jars' file identity and options, and only when
// there are any, so settings without plugins keep their identity.
func (s KotlinSettings) Identity() string {
	if s.IsZero() {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "kotlin-settings-v%d", kotlinSettingsIdentityVersion)
	fmt.Fprintf(&b, "|lang=%s|api=%s|progressive=%t", s.LanguageVersion, s.APIVersion, s.Progressive)
	b.WriteString("|optin=")
	b.WriteString(strings.Join(sortedUnique(s.OptIns), "\x1f"))
	b.WriteString("|flags=")
	b.WriteString(strings.Join(s.Flags, "\x1f"))
	b.WriteString(s.Plugins.identity())
	return b.String()
}

// IdentityHash is a short stable digest of Identity, "" for the zero value.
func (s KotlinSettings) IdentityHash() string {
	id := s.Identity()
	if id == "" {
		return ""
	}
	return hashutil.HashHex([]byte(id))[:16]
}

// CompilerArgs renders the settings as compiler arguments in the equals form
// the JVM helpers parse (-language-version=2.0, -opt-in=x, -progressive, -Xfoo).
// This is the one wire representation: ModuleSpec.CompilerArgs, the one-shot
// --kotlin-arg flags and the daemon request field all carry it.
func (s KotlinSettings) CompilerArgs() []string {
	var args []string
	if s.LanguageVersion != "" {
		args = append(args, "-language-version="+s.LanguageVersion)
	}
	if s.APIVersion != "" {
		args = append(args, "-api-version="+s.APIVersion)
	}
	for _, optIn := range sortedUnique(s.OptIns) {
		args = append(args, "-opt-in="+optIn)
	}
	if s.Progressive {
		args = append(args, "-progressive")
	}
	return append(args, s.Flags...)
}

// Describe is a one-line human summary for verbose output.
func (s KotlinSettings) Describe() string {
	if s.IsZero() {
		return "embedded compiler defaults"
	}
	args := s.CompilerArgs()
	if !s.Plugins.IsZero() {
		args = append(args, "compiler plugins: "+s.Plugins.Describe())
	}
	return strings.Join(args, " ")
}

var (
	versionRe     = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)
	pluginMinorRe = regexp.MustCompile(`^([0-9]+)\.([0-9]+)`)
	optInRe       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)
	flagValueRe   = regexp.MustCompile(`^[A-Za-z0-9_.,:+@\-]+$`)
	languageRe    = regexp.MustCompile(`^-XXLanguage:[+-][A-Za-z][A-Za-z0-9_]*$`)
)

// MinSupportedKotlinLanguage is the oldest language (and API) level the
// embedded compiler can analyze under. The helpers drop anything older and fall
// back to their default, so Go reports and identity-hashes only what they apply.
// Keep it equal to LanguageVersion.FIRST_SUPPORTED of the embedded compiler.
const MinSupportedKotlinLanguage = "2.0"

// compareKotlinVersions orders two major.minor versions; -1, 0 or 1.
func compareKotlinVersions(a, b string) int {
	parse := func(v string) (int, int) {
		m := pluginMinorRe.FindStringSubmatch(v)
		if m == nil {
			return 0, 0
		}
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		return major, minor
	}
	am, an := parse(a)
	bm, bn := parse(b)
	switch {
	case am != bm:
		if am < bm {
			return -1
		}
		return 1
	case an != bn:
		if an < bn {
			return -1
		}
		return 1
	}
	return 0
}

// kotlinFlagKind says whether an allowlisted flag takes a value.
type kotlinFlagKind int

const (
	flagBoolean kotlinFlagKind = iota
	flagValued
)

// kotlinFlagAllowlist is the authoritative set of -X flags krit applies from a
// Gradle model. It lists flags that change language semantics, type
// resolution or diagnostics the frontend produces; codegen, output, plugin and
// path flags are never applied. The JVM helpers keep an identical copy
// (KotlinCompilerSettings.kt in krit-fir) as defense in depth; a Go test
// keeps the lists equal.
var kotlinFlagAllowlist = map[string]kotlinFlagKind{
	"-Xannotation-default-target":                    flagValued,
	"-Xannotation-target-all":                        flagBoolean,
	"-Xcontext-parameters":                           flagBoolean,
	"-Xcontext-receivers":                            flagBoolean,
	"-Xcontext-sensitive-resolution":                 flagBoolean,
	"-Xconsistent-data-class-copy-visibility":        flagBoolean,
	"-Xdata-flow-based-exhaustiveness":               flagBoolean,
	"-Xenhance-type-parameter-types-to-def-not-null": flagBoolean,
	"-Xexplicit-backing-fields":                      flagBoolean,
	"-Xexpect-actual-classes":                        flagBoolean,
	"-Xjsr305":                                       flagValued,
	"-Xjvm-default":                                  flagValued,
	"-Xmulti-dollar-interpolation":                   flagBoolean,
	"-Xnested-type-aliases":                          flagBoolean,
	"-Xnon-local-break-continue":                     flagBoolean,
	"-Xnullability-annotations":                      flagValued,
	"-Xreturn-value-checker":                         flagValued,
	"-Xtype-enhancement-improvements-strict-mode":    flagBoolean,
	"-Xunrestricted-builder-inference":               flagBoolean,
	"-Xwhen-guards":                                  flagBoolean,
}

// KotlinFlagAllowlist returns the allowlisted flag names, sorted.
func KotlinFlagAllowlist() []string {
	names := make([]string, 0, len(kotlinFlagAllowlist))
	for name := range kotlinFlagAllowlist {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// protectedKotlinArgs are arguments krit owns: output, classpath, plugin, JDK,
// module-identity, friend-path and HMPP fragment settings. A model can never
// set them. The valued ones consume the following token in split form.
var protectedKotlinArgs = map[string]bool{
	"-d": true, "-classpath": true, "-cp": true, "-jvm-target": true, "-module-name": true,
	"-P": true, "-jdk-home": true, "-Xfriend-paths": true, "-Xplugin": true, "-Xjdk-release": true,
	"-Xadd-modules": true, "-Xbuild-file": true, "-Xmodule-path": true, "-Xmulti-platform": true,
	"-Xcommon-sources": true, "-Xexpect-actual-linker": true, "-no-stdlib": true, "-no-reflect": true,
	"-Xno-stdlib": true, "-Xno-reflect": true, "-Xcompiler-plugin": true, "-script-templates": true,
}

// protectedValued are the protected arguments whose value is the next token in
// split form (-d dir); everything else protected is a standalone flag.
var protectedValued = map[string]bool{
	"-d": true, "-classpath": true, "-cp": true, "-jvm-target": true, "-module-name": true,
	"-P": true, "-jdk-home": true, "-script-templates": true,
}

// IsProtectedKotlinArg reports whether arg names a krit-managed setting.
func IsProtectedKotlinArg(arg string) bool {
	name := arg
	if i := strings.IndexByte(name, '='); i >= 0 {
		name = name[:i]
	}
	return protectedKotlinArgs[name] || strings.HasPrefix(name, "-Xfragment") || strings.HasPrefix(name, "-Xplugin")
}

// parsedKotlinArgs is the classification of one freeCompilerArgs list.
type parsedKotlinArgs struct {
	languageVersion string
	apiVersion      string
	optIns          []string
	progressive     bool
	flags           []string
	rejected        []string
}

// kotlinArgParser walks one argument list, classifying each argument.
type kotlinArgParser struct {
	args     []string
	i        int
	out      parsedKotlinArgs
	seenFlag map[string]int
}

func (p *kotlinArgParser) reject(arg, reason string) {
	p.out.rejected = append(p.out.rejected, arg+" ("+reason+")")
}

// operand returns the value of a valued option in equals form (value) or split
// form (the next token).
func (p *kotlinArgParser) operand(value string, hasValue bool) (string, bool) {
	if hasValue {
		return value, true
	}
	if p.i+1 < len(p.args) {
		p.i++
		return strings.TrimSpace(p.args[p.i]), true
	}
	return "", false
}

func (p *kotlinArgParser) version(arg, name, value string, hasValue bool) {
	v, ok := p.operand(value, hasValue)
	if !ok || !versionRe.MatchString(v) {
		p.reject(arg, "unsupported version value")
		return
	}
	if name == "-language-version" {
		p.out.languageVersion = v
	} else {
		p.out.apiVersion = v
	}
}

// progressive accepts -progressive and -progressive=true|false; the latter
// turns progressive mode off, as in the compiler's boolean flag syntax.
func (p *kotlinArgParser) progressive(arg, value string, hasValue bool) {
	switch {
	case !hasValue || value == "true":
		p.out.progressive = true
	case value == "false":
		p.out.progressive = false
	default:
		p.reject(arg, "flag takes true or false")
	}
}

func (p *kotlinArgParser) optIn(arg, value string, hasValue bool) {
	v, ok := p.operand(value, hasValue)
	if !ok || !optInRe.MatchString(v) {
		p.reject(arg, "unsupported opt-in marker")
		return
	}
	p.out.optIns = append(p.out.optIns, v)
}

func (p *kotlinArgParser) protected(arg, name string, hasValue bool) {
	if protectedValued[name] && !hasValue && p.i+1 < len(p.args) {
		p.i++
		p.reject(arg+" "+p.args[p.i], "managed by krit")
		return
	}
	p.reject(arg, "managed by krit")
}

func (p *kotlinArgParser) flag(arg, name, value string, hasValue bool) {
	kind, ok := kotlinFlagAllowlist[name]
	switch {
	case !ok:
		p.reject(arg, "not an analysis-semantic setting krit applies")
	case kind == flagBoolean && hasValue:
		p.reject(arg, "flag takes no value")
	case kind == flagValued && (!hasValue || !flagValueRe.MatchString(value)):
		p.reject(arg, "flag needs a simple value")
	default:
		// A repeated flag keeps its last position, so contradictory
		// -XXLanguage toggles resolve the way the compiler would.
		p.out.flags = appendFlag(p.out.flags, p.seenFlag, name, arg)
	}
}

func (p *kotlinArgParser) one(arg string) {
	name, value, hasValue := strings.Cut(arg, "=")
	switch {
	case name == "-language-version" || name == "-api-version":
		p.version(arg, name, value, hasValue)
	case name == "-opt-in" || name == "-Xopt-in":
		p.optIn(arg, value, hasValue)
	case name == "-progressive":
		p.progressive(arg, value, hasValue)
	case IsProtectedKotlinArg(arg):
		p.protected(arg, name, hasValue)
	case languageRe.MatchString(arg):
		p.out.flags = appendFlag(p.out.flags, p.seenFlag, arg, arg)
	default:
		p.flag(arg, name, value, hasValue)
	}
}

// parseKotlinArgs classifies compiler arguments. Every argument ends up either
// applied (language/api version, opt-in, progressive, an allowlisted flag) or in
// rejected with a reason; none is dropped silently.
func parseKotlinArgs(args []string) parsedKotlinArgs {
	p := &kotlinArgParser{args: args, seenFlag: map[string]int{}}
	for ; p.i < len(args); p.i++ {
		if arg := strings.TrimSpace(args[p.i]); arg != "" {
			p.one(arg)
		}
	}
	return p.out
}

// appendFlag adds arg, first removing an earlier flag with the same key (the
// flag name, or for -XXLanguage:+X / -XXLanguage:-X the feature X), so the last
// occurrence wins and keeps its position.
func appendFlag(flags []string, seen map[string]int, key, arg string) []string {
	if feature, ok := strings.CutPrefix(arg, "-XXLanguage:"); ok {
		key = "-XXLanguage:" + feature[1:]
	}
	if at, ok := seen[key]; ok {
		flags = append(flags[:at], flags[at+1:]...)
		for k, v := range seen {
			if v > at {
				seen[k] = v - 1
			}
		}
	}
	seen[key] = len(flags)
	return append(flags, arg)
}

// ClassifyKotlinArgs applies the same classification the model resolver uses to
// an arbitrary argument list, returning the applied settings and the rejected
// arguments. The Kotlin helpers' parity tests and Go callers building module
// requests from other sources share it.
func ClassifyKotlinArgs(args []string) KotlinSettings {
	p := parseKotlinArgs(args)
	return KotlinSettings{
		LanguageVersion: p.languageVersion,
		APIVersion:      p.apiVersion,
		OptIns:          sortedUnique(p.optIns),
		Progressive:     p.progressive,
		Flags:           p.flags,
		Rejected:        p.rejected,
	}
}

// ResolveKotlinSettings computes the effective settings of one source set.
//
// A nil opts (a schema 1 model, or a source set the exporter had no Kotlin
// compile task for) yields the zero settings plus a note: the embedded
// compiler's defaults apply and that is reported, not hidden. When opts
// carries no explicit languageVersion, the Kotlin Gradle plugin's own
// major.minor is used, because that is the language version Gradle compiled the
// project with; apiVersion defaults to the language version, as in Kotlin.
func ResolveKotlinSettings(opts *KotlinOptions) KotlinSettings {
	if opts == nil {
		return KotlinSettings{Notes: []string{"no Kotlin compiler settings in the Gradle model; the embedded compiler's defaults apply"}}
	}
	parsed := parseKotlinArgs(opts.FreeCompilerArgs)
	var s KotlinSettings
	s.Rejected = parsed.rejected
	s.Flags = parsed.flags
	s.Progressive = opts.ProgressiveMode || parsed.progressive

	// Free compiler arguments are appended after the DSL values by Gradle, so a
	// -language-version there wins.
	language := firstNonEmpty(parsed.languageVersion, opts.LanguageVersion)
	switch {
	case language != "" && !versionRe.MatchString(language):
		s.Notes = append(s.Notes, fmt.Sprintf("languageVersion %q is not a major.minor version; ignored", language))
		language = ""
	case language == "":
		if m := pluginMinorRe.FindStringSubmatch(opts.PluginVersion); m != nil {
			language = m[1] + "." + m[2]
			s.Notes = append(s.Notes, fmt.Sprintf("languageVersion not configured; using %s from Kotlin Gradle plugin %s", language, opts.PluginVersion))
		} else {
			s.Notes = append(s.Notes, "languageVersion not configured and the Kotlin plugin version is unknown; the embedded compiler default applies")
		}
	}
	if language != "" && compareKotlinVersions(language, MinSupportedKotlinLanguage) < 0 {
		s.Notes = append(s.Notes, fmt.Sprintf("languageVersion %s is not supported by the embedded compiler (%s or newer); its default applies", language, MinSupportedKotlinLanguage))
		language = ""
	}
	s.LanguageVersion = language

	api := firstNonEmpty(parsed.apiVersion, opts.APIVersion)
	switch {
	case api != "" && !versionRe.MatchString(api):
		s.Notes = append(s.Notes, fmt.Sprintf("apiVersion %q is not a major.minor version; ignored", api))
		api = ""
	case api == "" && language != "":
		api = language
		s.Notes = append(s.Notes, "apiVersion not configured; defaults to the language version "+language)
	}
	if api != "" && compareKotlinVersions(api, MinSupportedKotlinLanguage) < 0 {
		s.Notes = append(s.Notes, fmt.Sprintf("apiVersion %s is not supported by the embedded compiler (%s or newer); its default applies", api, MinSupportedKotlinLanguage))
		api = ""
	}
	if api != "" && language != "" && compareKotlinVersions(api, language) > 0 {
		s.Notes = append(s.Notes, fmt.Sprintf("apiVersion %s is greater than languageVersion %s; not applied, the API level follows the language version", api, language))
		api = ""
	}
	s.APIVersion = api

	var optIns []string
	for _, v := range opts.OptIn {
		if optInRe.MatchString(v) {
			optIns = append(optIns, v)
		} else {
			s.Rejected = append(s.Rejected, "optIn "+v+" (unsupported opt-in marker)")
		}
	}
	s.OptIns = sortedUnique(append(optIns, parsed.optIns...))

	plugins, notes, rejected := ResolveCompilerPlugins(opts.CompilerPluginClasspath, opts.CompilerPluginOptions)
	s.Plugins = plugins
	s.Notes = append(s.Notes, notes...)
	s.Rejected = append(s.Rejected, rejected...)
	return s
}

// MergeKotlinSettings combines the settings of several source sets for a
// compilation that spans all of them (the whole-repo oracle and FIR passes).
// labels names each set for notes. Versions and progressive mode are applied
// only when every source set agrees; otherwise they stay at the embedded
// compiler's default with a note. Semantic flags use the intersection. Opt-ins
// use a sorted union to avoid false experimental-API errors, at the cost of
// potentially hiding a missing opt-in in another source set.
func MergeKotlinSettings(sets []KotlinSettings, labels []string) KotlinSettings {
	if len(sets) == 0 {
		return KotlinSettings{}
	}
	if len(sets) == 1 {
		return sets[0]
	}
	var out KotlinSettings
	label := func(i int) string {
		if i < len(labels) {
			return labels[i]
		}
		return fmt.Sprintf("#%d", i)
	}
	uniform := func(field string, get func(KotlinSettings) string) string {
		first := get(sets[0])
		for i := 1; i < len(sets); i++ {
			if get(sets[i]) != first {
				seen := map[string][]string{}
				var order []string
				for j := range sets {
					v := get(sets[j])
					if _, ok := seen[v]; !ok {
						order = append(order, v)
					}
					seen[v] = append(seen[v], label(j))
				}
				var parts []string
				for _, v := range order {
					if v == "" {
						v = "(unset)"
					}
					parts = append(parts, fmt.Sprintf("%s=%s in %s", field, v, summarizeLabels(seen[v])))
				}
				out.Notes = append(out.Notes, field+" differs across source sets ("+strings.Join(parts, "; ")+
					"); not applied, the embedded compiler default is used for the whole-repo compilation")
				return ""
			}
		}
		return first
	}
	out.LanguageVersion = uniform("languageVersion", func(s KotlinSettings) string { return s.LanguageVersion })
	out.APIVersion = uniform("apiVersion", func(s KotlinSettings) string { return s.APIVersion })
	progressive := uniform("progressive", func(s KotlinSettings) string { return fmt.Sprint(s.Progressive) })
	out.Progressive = progressive == "true"

	var pluginNotes []string
	out.Plugins, pluginNotes = mergeCompilerPlugins(sets)
	out.Notes = append(out.Notes, pluginNotes...)

	for _, settings := range sets {
		out.OptIns = append(out.OptIns, settings.OptIns...)
	}
	out.OptIns = sortedUnique(out.OptIns)
	for index, settings := range sets {
		if len(sortedUnique(settings.OptIns)) < len(out.OptIns) {
			out.Notes = append(out.Notes, fmt.Sprintf("opt-ins differ across source sets (e.g. %s is missing opt-ins enabled elsewhere); all opt-ins enabled by any source set are applied to avoid false experimental-API errors; missing opt-ins in other source sets may be hidden", label(index)))
			break
		}
	}
	out.Flags = intersect(sets, func(s KotlinSettings) []string { return s.Flags })
	for index, settings := range sets {
		if len(settings.Flags) > len(out.Flags) {
			out.Notes = append(out.Notes, fmt.Sprintf("flags differ across source sets (e.g. %s); only those enabled by every source set are applied", label(index)))
			break
		}
	}
	// Surface what was not applied exactly once.
	seen := map[string]bool{}
	for _, s := range sets {
		for _, r := range s.Rejected {
			if !seen[r] {
				seen[r] = true
				out.Rejected = append(out.Rejected, r)
			}
		}
		for _, n := range s.Notes {
			if !seen[n] {
				seen[n] = true
				out.Notes = append(out.Notes, n)
			}
		}
	}
	sort.Strings(out.Rejected)
	return out
}

func summarizeLabels(labels []string) string {
	if len(labels) <= 3 {
		return strings.Join(labels, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(labels[:3], ", "), len(labels)-3)
}

// intersect returns the elements every set contains, in the first set's order.
func intersect(sets []KotlinSettings, get func(KotlinSettings) []string) []string {
	var out []string
	for _, v := range get(sets[0]) {
		everywhere := true
		for _, s := range sets[1:] {
			found := false
			for _, o := range get(s) {
				if o == v {
					found = true
					break
				}
			}
			if !found {
				everywhere = false
				break
			}
		}
		if everywhere {
			out = append(out, v)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func sortedUnique(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	n := 0
	for i, v := range out {
		if i == 0 || v != out[i-1] {
			out[n] = v
			n++
		}
	}
	return out[:n]
}

// SourceSetKotlinSettings is the resolved settings of one source set, keyed so
// a per-module request can never pick another module's entry by accident.
type SourceSetKotlinSettings struct {
	Project   string
	SourceSet string
	Settings  KotlinSettings
}

// KotlinSettingsBySourceSet resolves every source set's settings in model order.
func (m *Model) KotlinSettingsBySourceSet() []SourceSetKotlinSettings {
	if m == nil {
		return nil
	}
	var out []SourceSetKotlinSettings
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			// A schema 2 source set with no `kotlin` object has no Kotlin compile
			// task (Java-only): it analyzes no Kotlin, so it neither sets nor vetoes
			// a setting. A schema 1 source set may simply predate the field, so it
			// stays "unknown": absent versions and flags are handled conservatively,
			// while declared opt-ins are still unioned across source sets.
			if s.Kotlin == nil && p.SchemaVersion >= 2 {
				continue
			}
			out = append(out, SourceSetKotlinSettings{Project: p.Path, SourceSet: s.Name, Settings: ResolveKotlinSettings(s.Kotlin)})
		}
	}
	return out
}

// EffectiveKotlinSettings merges every source set for a whole-repo compilation
// (see MergeKotlinSettings). An empty model yields the zero settings.
func (m *Model) EffectiveKotlinSettings() KotlinSettings {
	return MergeSourceSetSettings(m.KotlinSettingsBySourceSet())
}

// MergeSourceSetSettings merges already-resolved source set settings, possibly
// from several models.
func MergeSourceSetSettings(entries []SourceSetKotlinSettings) KotlinSettings {
	if len(entries) == 0 {
		return KotlinSettings{}
	}
	sets := make([]KotlinSettings, len(entries))
	labels := make([]string, len(entries))
	for i, e := range entries {
		sets[i] = e.Settings
		labels[i] = e.Project + ":" + e.SourceSet
	}
	return MergeKotlinSettings(sets, labels)
}
