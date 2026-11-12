package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	shippedconfig "github.com/kaeawc/krit/config"
	"gopkg.in/yaml.v3"
)

// DetektConfigFilenames lists the detekt config locations krit probes, in
// order, in each directory it walks while auto-discovering a detekt config.
var DetektConfigFilenames = []string{
	filepath.Join("config", "detekt", "detekt.yml"),
	"detekt.yml",
	".detekt.yml",
}

// detektBaselineFilename is the baseline detekt's Gradle plugin writes next to
// the config file by convention.
const detektBaselineFilename = "baseline.xml"

// LoadOptions are caller-supplied overrides for config loading; the zero value
// means "use krit.yml and auto-discovery".
type LoadOptions struct {
	// DetektConfigFile forces a detekt config path and wins over krit.yml.
	DetektConfigFile string
	// NoDetekt skips detekt import entirely, including auto-discovery.
	NoDetekt bool
}

// DetektImport describes what the detekt layer contributed to a Config.
type DetektImport struct {
	// Source is the detekt config file that was imported.
	Source string
	// Baseline is a detekt baseline file to apply when --baseline is unset.
	Baseline string
	// Notes lists detekt keys that could not be mapped.
	Notes []string
	// Warning reports an auto-discovered detekt config that could not be
	// read or parsed and was skipped.
	Warning string
}

// RegistryView lets the detekt importer ask about krit's registered rules
// without importing the rules package (which imports this one).
type RegistryView interface {
	// HasRuleSet reports whether krit has a ruleset of this name.
	HasRuleSet(set string) bool
	// RuleSetOf returns the ruleset krit registers rule under.
	RuleSetOf(rule string) (string, bool)
	// CanonicalOption resolves an option key or alias of rule to its primary
	// name; ok is false when the rule has no such option.
	CanonicalOption(rule, key string) (string, bool)
}

var registryView RegistryView

// SetRegistryView installs the rule registry the importer validates against.
// Without one the importer maps names only and cannot drop unknown keys.
func SetRegistryView(v RegistryView) { registryView = v }

// Detekt returns what the detekt layer imported, or nil when none was used.
func (c *Config) Detekt() *DetektImport {
	if c == nil {
		return nil
	}
	return c.detekt
}

// detektRuleSets are the detekt rulesets krit maps; other top-level detekt
// sections (build, processors, formatting, ...) are reported and skipped.
var detektRuleSets = map[string]bool{
	"comments": true, "complexity": true, "coroutines": true, "empty-blocks": true,
	"exceptions": true, "naming": true, "performance": true, "potential-bugs": true,
	"style": true,
}

// detektRuleRenames maps detekt rule names to the krit rules that replace them.
var detektRuleRenames = map[string][]string{
	"ComplexMethod":               {"CyclomaticComplexMethod"},
	"FunctionName":                {"FunctionNaming"},
	"ClassName":                   {"ClassNaming"},
	"VariableName":                {"VariableNaming"},
	"PackageName":                 {"PackageNaming"},
	"FunctionMaxLength":           {"FunctionNameMaxLength"},
	"FunctionMinLength":           {"FunctionNameMinLength"},
	"PackageDirectoryMismatch":    {"InvalidPackageDeclaration"},
	"EmptyKtFile":                 {"EmptyKotlinFile"},
	"CommentOverPrivateFunction":  {"DocumentationOverPrivateFunction"},
	"CommentOverPrivateProperty":  {"DocumentationOverPrivateProperty"},
	"MandatoryBracesIfStatements": {"BracesOnIfStatements"},
	"UnnecessaryAbstractClass":    {"AbstractClassCanBeConcreteClass", "AbstractClassCanBeInterface"},

	// detekt style rules whose krit id differs.
	"MayBeConst":                      {"MayBeConstant"},
	"UnusedImports":                   {"UnusedImport"},
	"SpacingBetweenPackageAndImports": {"SpacingAfterPackageAndImports"},
	"RedundantVisibilityModifierRule": {"RedundantVisibilityModifier"},
	"OptionalWhenBraces":              {"BracesOnWhenStatements"},
	// detekt's UnusedPrivateMember covers private functions; classes and properties have their own rules.
	"UnusedPrivateMember": {"UnusedPrivateFunction"},
}

// detektThreshold renames a detekt 1.x "report when value >= N" option to the
// krit "report when value > allowed" option, so the value shifts by one.
type detektThreshold struct{ Target string }

// detektThresholds is keyed by krit rule id, then by the detekt option name. NamedArguments and
// NestedScopeFunctions are absent because detekt reports them at value > threshold, like krit.
var detektThresholds = map[string]map[string]detektThreshold{
	"CyclomaticComplexMethod": {"threshold": {"allowedComplexity"}},
	"CognitiveComplexMethod":  {"threshold": {"allowedComplexity"}},
	"ComplexCondition":        {"threshold": {"allowedConditions"}},
	"ComplexInterface":        {"threshold": {"allowedDefinitions"}},
	"CouldBeSequence":         {"threshold": {"allowedOperations"}},
	"LargeClass":              {"threshold": {"allowedLines"}},
	"LongMethod":              {"threshold": {"allowedLines"}},
	"LongParameterList": {
		"functionThreshold":    {"allowedFunctionParameters"},
		"constructorThreshold": {"allowedConstructorParameters"},
	},
	"MethodOverloading":        {"threshold": {"allowedOverloads"}},
	"NestedBlockDepth":         {"threshold": {"allowedDepth"}},
	"StringLiteralDuplication": {"threshold": {"allowedDuplications"}},
	"TooManyFunctions": {
		"thresholdInFiles":      {"allowedFunctionsPerFile"},
		"thresholdInClasses":    {"allowedFunctionsPerClass"},
		"thresholdInInterfaces": {"allowedFunctionsPerInterface"},
		"thresholdInObjects":    {"allowedFunctionsPerObject"},
		"thresholdInEnums":      {"allowedFunctionsPerEnum"},
	},
}

// ruleGenericKeys are per-rule keys krit interprets for every rule.
var ruleGenericKeys = map[string]bool{"active": true, "excludes": true, "includes": true, "ignoreAnnotated": true}

// DetektThresholdOptions returns the detekt-name to krit-name threshold map for
// a krit rule id, for the registry consistency test.
func DetektThresholdOptions() map[string]map[string]string {
	out := make(map[string]map[string]string, len(detektThresholds))
	for rule, opts := range detektThresholds {
		m := make(map[string]string, len(opts))
		for k, v := range opts {
			m[k] = v.Target
		}
		out[rule] = m
	}
	return out
}

// DetektRuleRenames returns a copy of the detekt-to-krit rule rename table.
func DetektRuleRenames() map[string][]string {
	out := make(map[string][]string, len(detektRuleRenames))
	for k, v := range detektRuleRenames {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// ParseDetektConfig translates detekt YAML into a krit config map plus notes
// for every key it could not map.
func ParseDetektConfig(data []byte, name string) (map[string]interface{}, []string, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("parsing detekt config %s: %w", name, err)
	}
	out, notes := translateDetekt(raw)
	return out, notes, nil
}

func translateDetekt(raw map[string]interface{}) (map[string]interface{}, []string) {
	out := map[string]interface{}{}
	var notes []string
	view := registryView

	for _, section := range slices.Sorted(maps.Keys(raw)) {
		body, ok := raw[section].(map[string]interface{})
		if !ok {
			notes = append(notes, fmt.Sprintf("detekt section %q ignored: not a ruleset", section))
			continue
		}
		if !detektRuleSets[section] || (view != nil && !view.HasRuleSet(section)) {
			notes = append(notes, fmt.Sprintf("detekt section %q ignored: no krit ruleset", section))
			continue
		}
		for _, key := range []string{"active", "excludes", "includes"} {
			if v, ok := body[key]; ok {
				setNested(out, v, section, key)
			}
		}
		// Renamed names go first so a rule configured under both names keeps the krit name's value.
		for _, pass := range []bool{true, false} {
			for _, rule := range slices.Sorted(maps.Keys(body)) {
				if ruleGenericKeys[rule] {
					continue
				}
				_, renamed := detektRuleRenames[rule]
				if renamed != pass {
					continue
				}
				opts, ok := body[rule].(map[string]interface{})
				if !ok {
					notes = append(notes, fmt.Sprintf("detekt %s.%s ignored: not a rule block", section, rule))
					continue
				}
				targets := detektRuleRenames[rule]
				if !renamed {
					targets = []string{rule}
				}
				for _, target := range targets {
					notes = translateRule(out, view, section, rule, target, opts, notes)
				}
			}
		}
	}
	return out, notes
}

// translateRule copies one detekt rule block into out under the krit rule.
func translateRule(out map[string]interface{}, view RegistryView, section, rule, target string, opts map[string]interface{}, notes []string) []string {
	set := section
	if view != nil {
		var ok bool
		if set, ok = view.RuleSetOf(target); !ok {
			return append(notes, fmt.Sprintf("detekt %s.%s ignored: no krit rule %s", section, rule, target))
		}
	}
	for _, key := range slices.Sorted(maps.Keys(opts)) {
		val := opts[key]
		if ruleGenericKeys[key] {
			setNested(out, val, set, target, key)
			continue
		}
		if th, ok := detektThresholds[target][key]; ok {
			setNested(out, shiftInt(val, -1), set, target, th.Target)
			continue
		}
		if view == nil {
			setNested(out, val, set, target, key)
			continue
		}
		canonical, ok := view.CanonicalOption(target, key)
		if !ok {
			notes = append(notes, fmt.Sprintf("detekt %s.%s.%s ignored: krit rule %s has no such option", section, rule, key, target))
			continue
		}
		setNested(out, val, set, target, canonical)
	}
	// Keep the rule block present so `active` of a bare rule is not lost.
	if _, ok := lookupMap(out, set, target); !ok {
		setNested(out, map[string]interface{}{}, set, target)
	}
	return notes
}

// shiftInt adds delta to integer values (floored at 0) and leaves others as is.
func shiftInt(v interface{}, delta int) interface{} {
	n, ok := v.(int)
	if !ok {
		return v
	}
	if n+delta < 0 {
		return 0
	}
	return n + delta
}

// setNested stores v at path inside m, creating intermediate maps.
func setNested(m map[string]interface{}, v interface{}, path ...string) {
	cur := m
	for _, k := range path[:len(path)-1] {
		next, ok := cur[k].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			cur[k] = next
		}
		cur = next
	}
	cur[path[len(path)-1]] = v
}

// canonicalizeAliases rewrites rule option aliases (for example `threshold`)
// to primary names so a higher layer's alias is not shadowed by a lower
// layer's primary key when the layers merge.
func canonicalizeAliases(data map[string]interface{}) {
	view := registryView
	if view == nil {
		return
	}
	for set, sv := range data {
		setMap, ok := sv.(map[string]interface{})
		if !ok {
			continue
		}
		for rule, rv := range setMap {
			ruleMap, ok := rv.(map[string]interface{})
			if !ok {
				continue
			}
			if owner, ok := view.RuleSetOf(rule); !ok || owner != set {
				continue
			}
			for key, val := range ruleMap {
				if ruleGenericKeys[key] {
					continue
				}
				canonical, ok := view.CanonicalOption(rule, key)
				if !ok || canonical == key {
					continue
				}
				if _, exists := ruleMap[canonical]; !exists {
					ruleMap[canonical] = val
				}
				delete(ruleMap, key)
			}
		}
	}
}

// detektSettings reads the optional top-level `detekt:` block of a krit config.
type detektSettings struct {
	enabled    bool
	configFile string
	baseline   string
}

func readDetektSettings(user *Config, userFile string) detektSettings {
	s := detektSettings{enabled: true}
	if user == nil {
		return s
	}
	if v, ok := lookupValue(user.data, "detekt", "enabled"); ok {
		if b, ok := toBool(v); ok {
			s.enabled = b
		}
	}
	s.configFile = resolveRelative(user.GetTopLevelString("detekt", "configFile", ""), userFile)
	s.baseline = resolveRelative(user.GetTopLevelString("detekt", "baseline", ""), userFile)
	return s
}

// resolveRelative anchors a relative path at the directory of the krit config
// that named it, or at the working directory when there is none.
func resolveRelative(path, kritFile string) string {
	if path == "" || filepath.IsAbs(path) || kritFile == "" {
		return path
	}
	return filepath.Join(filepath.Dir(kritFile), path)
}

// loadDetektLayer finds and translates the detekt config for this load. It
// returns a nil map when detekt import is off or no config exists.
func loadDetektLayer(user *Config, userFile string, roots []string, opts LoadOptions) (map[string]interface{}, *DetektImport, error) {
	settings := readDetektSettings(user, userFile)
	if opts.NoDetekt || (opts.DetektConfigFile == "" && !settings.enabled) {
		return nil, nil, nil
	}
	path := opts.DetektConfigFile
	if path == "" {
		path = settings.configFile
	}
	explicit := path != ""
	if !explicit {
		dirs := autoDetectDirs(roots)
		if userFile != "" {
			dirs = append(dirs, filepath.Dir(userFile))
		}
		path = discoverDetektConfig(dirs)
	}
	if path == "" {
		if settings.baseline != "" {
			return nil, &DetektImport{Baseline: settings.baseline}, nil
		}
		return nil, nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if explicit {
			return nil, nil, fmt.Errorf("reading detekt config: %w", err)
		}
		return nil, nil, nil
	}
	layer, notes, err := ParseDetektConfig(data, path)
	if err != nil {
		if explicit {
			return nil, nil, err
		}
		// A discovered detekt config is optional, so a broken one must not
		// discard the krit config it would have been layered beneath.
		return nil, &DetektImport{Baseline: settings.baseline, Warning: fmt.Sprintf("ignoring detekt config: %v", err)}, nil
	}
	if defaults, _, derr := ParseDetektConfig(shippedconfig.DetektDefaults(), shippedconfig.DetektDefaultsName); derr == nil {
		// detekt applies its own defaults to whatever the project leaves out.
		layer = mergeMaps(defaults, layer)
	}
	info := &DetektImport{Source: path, Baseline: settings.baseline, Notes: notes}
	if info.Baseline == "" {
		sibling := filepath.Join(filepath.Dir(path), detektBaselineFilename)
		if fi, err := os.Stat(sibling); err == nil && !fi.IsDir() {
			info.Baseline = sibling
		}
	}
	return layer, info, nil
}

// discoverDetektConfig walks up from each directory to the VCS root (or the
// home directory) and returns the first detekt config it finds.
func discoverDetektConfig(dirs []string) string {
	seen := map[string]struct{}{}
	home, _ := os.UserHomeDir()
	for _, start := range dirs {
		abs, err := filepath.Abs(start)
		if err != nil {
			abs = start
		}
		for dir := abs; ; {
			if _, visited := seen[dir]; visited || (home != "" && dir == home) {
				break
			}
			seen[dir] = struct{}{}
			for _, name := range DetektConfigFilenames {
				candidate := filepath.Join(dir, name)
				if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
					return candidate
				}
			}
			parent := filepath.Dir(dir)
			if IsVCSRoot(dir) || parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

// Summary renders the import as human-readable lines for --verbose output.
func (d *DetektImport) Summary() []string {
	if d == nil {
		return nil
	}
	var lines []string
	if d.Source != "" {
		lines = append(lines, "imported detekt config "+d.Source)
	}
	if d.Baseline != "" {
		lines = append(lines, "detekt baseline "+d.Baseline)
	}
	for _, n := range d.Notes {
		lines = append(lines, strings.TrimSpace(n))
	}
	return lines
}
