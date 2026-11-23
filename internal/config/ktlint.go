package config

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ktlintRuleEquivalents maps ktlint standard rule IDs to the krit rules that
// check the same thing. A project that disables the ktlint rule in
// .editorconfig (`ktlint_standard_no-wildcard-imports = disabled`) has
// chosen the style the krit rule would report.
var ktlintRuleEquivalents = map[string][]string{
	"no-wildcard-imports":  {"WildcardImport"},
	"max-line-length":      {"MaxLineLength"},
	"filename":             {"MatchingDeclarationName"},
	"function-naming":      {"FunctionNaming"},
	"package-name":         {"PackageNaming"},
	"class-naming":         {"ClassNaming"},
	"enum-entry-name-case": {"EnumNaming"},
	"final-newline":        {"NewLineAtEndOfFile"},
	"no-trailing-spaces":   {"TrailingWhitespace"},
	"no-unused-imports":    {"UnusedImport"},
	"modifier-order":       {"ModifierOrder"},
	"no-empty-file":        {"EmptyKotlinFile"},
	"no-empty-class-body":  {"EmptyClassBlock"},
}

// KtlintImport records what the .editorconfig ktlint settings contributed.
type KtlintImport struct {
	// Source is the closest .editorconfig that had ktlint settings.
	Source string
	// Disabled are the krit rules turned off by disabled ktlint rules.
	Disabled []string
	// StarImportPackages are ij_kotlin_packages_to_use_import_on_demand
	// entries added to WildcardImport's excludeImports.
	StarImportPackages []string
	// MaxLineLength is the max_line_length applied to MaxLineLength: a
	// positive limit, -1 for "off", 0 when unset.
	MaxLineLength int
}

// loadKtlintLayer reads the ktlint settings of the .editorconfig files above
// the scan roots: disabled ktlint rules turn off their krit equivalents, the
// packages IntelliJ imports on demand are exempt from WildcardImport, and
// max_line_length sets MaxLineLength's limit.
// It returns a nil layer when there is nothing to apply.
func loadKtlintLayer(roots []string) (map[string]interface{}, *KtlintImport) {
	props, source := ktlintEditorConfigProps(roots)
	if len(props) == 0 {
		return nil, nil
	}
	view := registryView
	layer := map[string]interface{}{}
	info := &KtlintImport{Source: source}
	for key, value := range props {
		if !strings.HasPrefix(key, "ktlint_standard_") || !strings.EqualFold(value, "disabled") {
			continue
		}
		for _, rule := range ktlintRuleEquivalents[strings.TrimPrefix(key, "ktlint_standard_")] {
			set, ok := ktlintRuleSet(view, rule)
			if !ok {
				continue
			}
			setNested(layer, false, set, rule, "active")
			info.Disabled = append(info.Disabled, rule)
		}
	}
	if v := props["ij_kotlin_packages_to_use_import_on_demand"]; v != "" {
		var pkgs []string
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				pkgs = append(pkgs, p)
			}
		}
		if set, ok := ktlintRuleSet(view, "WildcardImport"); ok && len(pkgs) > 0 {
			setNested(layer, pkgs, set, "WildcardImport", "excludeImports")
			info.StarImportPackages = pkgs
		}
	}
	applyEditorConfigLineLength(layer, info, view, props["max_line_length"])
	if len(layer) == 0 {
		return nil, nil
	}
	sort.Strings(info.Disabled)
	return layer, info
}

// applyEditorConfigLineLength sets MaxLineLength from max_line_length, the
// line length the project's formatter (ktlint, IntelliJ) enforces.
func applyEditorConfigLineLength(layer map[string]interface{}, info *KtlintImport, view RegistryView, value string) {
	set, ok := ktlintRuleSet(view, "MaxLineLength")
	if !ok || value == "" {
		return
	}
	if strings.EqualFold(value, "off") {
		setNested(layer, false, set, "MaxLineLength", "active")
		info.MaxLineLength = -1
		return
	}
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		setNested(layer, n, set, "MaxLineLength", "maxLineLength")
		info.MaxLineLength = n
	}
}

func ktlintRuleSet(view RegistryView, rule string) (string, bool) {
	if view == nil {
		return "style", true
	}
	return view.RuleSetOf(rule)
}

// ktlintEditorConfigProps merges the ktlint_* and
// ij_kotlin_packages_to_use_import_on_demand properties of the
// .editorconfig files from the first root up to a `root = true` file,
// closest winning, and returns the closest file that had any.
func ktlintEditorConfigProps(roots []string) (map[string]string, string) {
	keep := func(key string) bool {
		return strings.HasPrefix(key, "ktlint_") || key == "ij_kotlin_packages_to_use_import_on_demand" || key == "max_line_length"
	}
	if len(roots) == 0 {
		roots = []string{"."}
	}
	dir, err := filepath.Abs(roots[0])
	if err != nil {
		return nil, ""
	}
	if info, err := os.Stat(dir); err != nil {
		return nil, ""
	} else if !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	merged := map[string]string{}
	source := ""
	for {
		path := filepath.Join(dir, ".editorconfig")
		props, isRoot := parseEditorConfigKeys(path, keep)
		if len(props) > 0 && source == "" {
			source = path
		}
		for k, v := range props {
			if _, closer := merged[k]; !closer {
				merged[k] = v
			}
		}
		parent := filepath.Dir(dir)
		if isRoot || parent == dir || IsVCSRoot(dir) {
			break
		}
		dir = parent
	}
	return merged, source
}

// Ktlint returns what the .editorconfig ktlint settings contributed, or nil.
func (c *Config) Ktlint() *KtlintImport {
	if c == nil {
		return nil
	}
	return c.ktlint
}
