// Command krit-gen reads the rule inventory (build/rule_inventory.json)
// and emits Meta() RuleDescriptor implementations for each rule it
// knows about. Output is grouped by the rule's file of origin — all
// rules defined in internal/rules/licensing.go become a single
// zz_meta_licensing_gen.go alongside it.
//
// Phase 2D of the CodegenRegistry migration uses this to prove the
// generator's output shape is sound for a single ruleset (licensing);
// Phase 3 will wire the emitted files into the build and drop the
// legacy switch/map plumbing.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	if err := Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "krit-gen:", err)
		os.Exit(1)
	}
}

// Run is the testable entry point. args excludes the binary name
// (os.Args[0]). stdout/stderr receive informational messages; errors are
// returned for the caller to format.
func Run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("krit-gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		inventoryPath = fs.String("inventory", "", "path to rule_inventory.json (required)")
		outDir        = fs.String("out", "internal/rules", "output directory for generated files")
		rulesetsCSV   = fs.String("rulesets", "", "comma-separated list of rulesets to emit; empty means all")
		pkgName       = fs.String("package", "rules", "Go package name for generated files")
		verify        = fs.Bool("verify", false, "if set, fail when regenerating would change existing files (for CI)")
		rootDir       = fs.String("root", "", "project root for computing source hashes; defaults to cwd")
		emitIndex     = fs.Bool("emit-index", true, "emit the AllMetaProviders index file (zz_meta_index_gen.go) after per-file Meta() output")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *inventoryPath == "" {
		return errors.New("-inventory is required")
	}

	root := *rootDir
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve cwd: %w", err)
		}
		root = cwd
	}

	inv, err := loadInventory(*inventoryPath)
	if err != nil {
		return fmt.Errorf("load inventory: %w", err)
	}

	wanted := parseRulesetsFilter(*rulesetsCSV)
	rules := filterRules(inv.Rules, wanted)
	if len(rules) == 0 {
		return fmt.Errorf("no rules matched filter %q", *rulesetsCSV)
	}

	// Group by file-of-origin, deterministically.
	byFile := groupByFile(rules)

	// Compute source hashes per source file once and share them with all
	// rules defined in that file.
	hashes := make(map[string]string, len(byFile))
	for srcFile := range byFile {
		abs := filepath.Join(root, srcFile)
		h, err := fileHash(abs)
		if err != nil {
			return fmt.Errorf("hash %s: %w", srcFile, err)
		}
		hashes[srcFile] = h
	}

	// Emit.
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", *outDir, err)
	}

	changed := 0
	for _, srcFile := range sortedKeys(byFile) {
		fileRules := byFile[srcFile]
		sort.SliceStable(fileRules, func(i, j int) bool {
			if fileRules[i].StructType != fileRules[j].StructType {
				return fileRules[i].StructType < fileRules[j].StructType
			}
			return fileRules[i].InitLine < fileRules[j].InitLine
		})
		// Deduplicate by struct_type: the same rule struct can be
		// registered under multiple IDs (migration aliases), but Go
		// allows only one Meta() method per type. Keep the first entry
		// for each struct (earliest InitLine after sort).
		fileRules = dedupeByStruct(fileRules)

		// Partition into "codegen-emits Meta()" and "hand-written Meta()".
		// The excluded set names structs whose Meta() is maintained
		// manually in a sibling meta_*.go file — because the legacy
		// config.go does something the generator cannot express as a list
		// of Options (multi-field writes, whole-config reads, value
		// transforms). The generator emits a NOTE comment for each
		// excluded struct so a future reader can find the hand-written
		// version.
		var excluded []ruleEntry
		kept := fileRules[:0]
		for _, r := range fileRules {
			if excludedStructs[r.StructType] {
				excluded = append(excluded, r)
				continue
			}
			kept = append(kept, r)
		}
		fileRules = kept

		outName := emittedFilename(srcFile)
		outPath := filepath.Join(*outDir, outName)

		// If nothing remains after exclusion, remove any stale generated
		// file and skip emission entirely.
		if len(fileRules) == 0 {
			if _, err := os.Stat(outPath); err == nil {
				if *verify {
					return fmt.Errorf("verify: %s is out of date (re-run krit-gen to delete)", outPath)
				}
				if err := os.Remove(outPath); err != nil {
					return fmt.Errorf("remove %s: %w", outPath, err)
				}
				changed++
			}
			continue
		}

		src, err := renderFile(*pkgName, srcFile, hashes[srcFile], fileRules, excluded)
		if err != nil {
			return fmt.Errorf("render %s: %w", srcFile, err)
		}

		if *verify {
			existing, readErr := os.ReadFile(outPath)
			if readErr != nil {
				return fmt.Errorf("verify %s: %w", outPath, readErr)
			}
			if string(existing) != string(src) {
				return fmt.Errorf("verify: %s is out of date (re-run krit-gen)", outPath)
			}
			continue
		}

		if err := writeIfChanged(outPath, src); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		changed++
	}

	// Emit the cross-file metaByName index. It references every rule
	// struct by zero-value pointer so downstream code can call Meta()
	// without depending on the Registry surface (which wraps some rules
	// in adapters that hide the concrete struct from Unwrap).
	if *emitIndex {
		indexPath := filepath.Join(*outDir, "zz_meta_index_gen.go")
		indexSrc, err := renderIndexFile(*pkgName, rules)
		if err != nil {
			return fmt.Errorf("render index: %w", err)
		}

		if *verify {
			existing, readErr := os.ReadFile(indexPath)
			if readErr != nil {
				return fmt.Errorf("verify %s: %w", indexPath, readErr)
			}
			if string(existing) != string(indexSrc) {
				return fmt.Errorf("verify: %s is out of date (re-run krit-gen)", indexPath)
			}
		} else {
			if err := writeIfChanged(indexPath, indexSrc); err != nil {
				return fmt.Errorf("write %s: %w", indexPath, err)
			}
			changed++
		}
	}

	if !*verify {
		fmt.Fprintf(stdout, "krit-gen: emitted %d file(s) in %s\n", changed, *outDir)
	}
	return nil
}

// excludedStructs names rule struct types whose Meta() is NOT emitted by
// the generator. Each of these lives in a hand-written meta_*.go sibling
// because the legacy config.go does something the generator cannot express
// as a list of Options:
//
//   - ForbiddenImportRule: forbiddenImports value writes BOTH
//     ForbiddenImports AND Patterns (a multi-field assignment).
//   - LayerDependencyViolationRule: reads the whole config tree via
//     arch.ParseLayerConfig; uses registry.CustomApply instead of Options.
//   - NewerVersionAvailableRule: value transform from []string to
//     []libMinVersion via parseRecommendedVersionSpecs.
//   - PublicToInternalLeakyAbstractionRule: value transform from int
//     percent to float64 fraction.
//
// When a rule is excluded, the generator still emits a NOTE comment in the
// file where its Meta() would have lived so reviewers can find the
// hand-written version.
var excludedStructs = map[string]bool{
	"ForbiddenImportRule":                  true,
	"LayerDependencyViolationRule":         true,
	"NewerVersionAvailableRule":            true,
	"PublicToInternalLeakyAbstractionRule": true,
}

// ----------------------------------------------------------------------
// Inventory types.

type inventory struct {
	GeneratedAt  string      `json:"generated_at"`
	SourceCommit string      `json:"source_commit"`
	Rules        []ruleEntry `json:"rules"`
}

type ruleEntry struct {
	StructType     string                 `json:"struct_type"`
	File           string                 `json:"file"`
	InitFile       string                 `json:"init_file"`
	InitLine       int                    `json:"init_line"`
	ID             string                 `json:"id"`
	Ruleset        string                 `json:"ruleset"`
	Severity       string                 `json:"severity"`
	Description    string                 `json:"description"`
	DefaultActive  bool                   `json:"default_active"`
	FixLevel       string                 `json:"fix_level"`
	Confidence     float64                `json:"confidence"`
	NodeTypes      []string               `json:"node_types"`
	Oracle         interface{}            `json:"oracle"`
	Needs          []string               `json:"needs"`
	StructDefaults map[string]interface{} `json:"struct_defaults"`
	Options        []optionEntry          `json:"options"`
	Warnings       []string               `json:"warnings"`
}

type optionEntry struct {
	Field       string      `json:"field"`
	GoType      string      `json:"go_type"`
	YamlKey     string      `json:"yaml_key"`
	Aliases     []string    `json:"aliases"`
	Default     interface{} `json:"default"`
	Description string      `json:"description"`
}

func loadInventory(path string) (*inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var inv inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

// ----------------------------------------------------------------------
// Filtering + grouping.

func parseRulesetsFilter(csv string) map[string]bool {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return nil
	}
	out := make(map[string]bool)
	for _, part := range strings.Split(csv, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		out[p] = true
	}
	return out
}

func filterRules(all []ruleEntry, wanted map[string]bool) []ruleEntry {
	if wanted == nil {
		return all
	}
	out := make([]ruleEntry, 0, len(all))
	for _, r := range all {
		if wanted[r.Ruleset] {
			out = append(out, r)
		}
	}
	return out
}

func groupByFile(rules []ruleEntry) map[string][]ruleEntry {
	out := make(map[string][]ruleEntry)
	for _, r := range rules {
		out[r.File] = append(out[r.File], r)
	}
	return out
}

// dedupeByStruct returns the first ruleEntry per distinct StructType. The
// input must already be sorted so the preferred entry comes first. When
// duplicates are collapsed, any aliased IDs are noted in the kept entry's
// Warnings so humans see the collision in the generated TODO comment.
func dedupeByStruct(rules []ruleEntry) []ruleEntry {
	seen := make(map[string]int, len(rules))
	out := make([]ruleEntry, 0, len(rules))
	for _, r := range rules {
		if idx, ok := seen[r.StructType]; ok {
			out[idx].Warnings = append(out[idx].Warnings,
				fmt.Sprintf("%s is also registered as rule ID %q; Meta() only represents the primary ID", r.StructType, r.ID))
			continue
		}
		seen[r.StructType] = len(out)
		out = append(out, r)
	}
	return out
}

func sortedKeys(m map[string][]ruleEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ----------------------------------------------------------------------
// Filename + hashing.

// emittedFilename maps internal/rules/licensing.go -> zz_meta_licensing_gen.go.
// Only the basename-without-extension is used, since the caller chose
// the output directory.
func emittedFilename(srcFile string) string {
	base := filepath.Base(srcFile)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	return "zz_meta_" + stem + "_gen.go"
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16], nil
}

// ----------------------------------------------------------------------
// Rendering.

// renderFile builds a gofmt'd Go source for the given group of rules
// that share a source file. excluded names any rule structs from this
// source file whose Meta() lives in a hand-written sibling file; the
// generator emits a NOTE comment for each so readers can find it.
func renderFile(pkgName, srcFile, srcHash string, rules []ruleEntry, excluded []ruleEntry) ([]byte, error) {
	var b strings.Builder
	b.WriteString("// Code generated by krit-gen. DO NOT EDIT.\n")
	fmt.Fprintf(&b, "// source: %s\n", srcFile)
	b.WriteString("// generator: internal/codegen/cmd/krit-gen\n")
	for _, ex := range excluded {
		fmt.Fprintf(&b, "// NOTE: Meta() for %s is hand-written in %s.\n",
			ex.StructType, handWrittenMetaFilename(ex.StructType))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "package %s\n\n", pkgName)

	needsRegex := anyRegexOption(rules)
	b.WriteString("import (\n")
	if needsRegex {
		b.WriteString("\t\"regexp\"\n\n")
	}
	b.WriteString("\t\"github.com/kaeawc/krit/internal/rules/registry\"\n")
	b.WriteString(")\n\n")

	if needsRegex {
		// Keep the compiler happy if no emitted literal ends up using
		// regexp after all — extremely unlikely given anyRegexOption,
		// but guard just in case.
		b.WriteString("var _ = regexp.MustCompile\n\n")
	}

	for i, r := range rules {
		if i > 0 {
			b.WriteString("\n")
		}
		renderRuleMeta(&b, srcHash, r)
	}

	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("gofmt generated source: %w\n---\n%s", err, b.String())
	}
	return formatted, nil
}

// handWrittenMetaFilename returns the conventional sibling filename for a
// hand-written Meta() — meta_<snake_case_struct>.go without the trailing
// "Rule" suffix. Keeps the NOTE comment pointer stable so reviewers can
// grep.
func handWrittenMetaFilename(structType string) string {
	name := strings.TrimSuffix(structType, "Rule")
	var out strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			r = r + ('a' - 'A')
		}
		out.WriteRune(r)
	}
	return "meta_" + out.String() + ".go"
}

func anyRegexOption(rules []ruleEntry) bool {
	for _, r := range rules {
		for _, o := range r.Options {
			if o.GoType == "regex" {
				return true
			}
		}
	}
	return false
}

func renderRuleMeta(b *strings.Builder, srcHash string, r ruleEntry) {
	for _, w := range r.Warnings {
		fmt.Fprintf(b, "// TODO(krit-gen): %s\n", w)
	}
	fmt.Fprintf(b, "func (r *%s) Meta() registry.RuleDescriptor {\n", r.StructType)
	b.WriteString("\treturn registry.RuleDescriptor{\n")
	fmt.Fprintf(b, "\t\tID:            %q,\n", r.ID)
	fmt.Fprintf(b, "\t\tRuleSet:       %q,\n", r.Ruleset)
	fmt.Fprintf(b, "\t\tSeverity:      %q,\n", r.Severity)
	fmt.Fprintf(b, "\t\tDescription:   %q,\n", r.Description)
	fmt.Fprintf(b, "\t\tDefaultActive: %t,\n", r.DefaultActive)
	fmt.Fprintf(b, "\t\tFixLevel:      %q,\n", r.FixLevel)
	fmt.Fprintf(b, "\t\tConfidence:    %s,\n", formatFloat(r.Confidence))
	fmt.Fprintf(b, "\t\tSourceHash:    %q,\n", srcHash)

	if len(r.Options) == 0 {
		b.WriteString("\t}\n")
		b.WriteString("}\n")
		return
	}

	b.WriteString("\t\tOptions: []registry.ConfigOption{\n")
	// Sort options by Name for deterministic output.
	opts := append([]optionEntry(nil), r.Options...)
	sort.SliceStable(opts, func(i, j int) bool {
		return opts[i].YamlKey < opts[j].YamlKey
	})
	for _, o := range opts {
		renderOption(b, r.StructType, o)
	}
	b.WriteString("\t\t},\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")
}

func renderOption(b *strings.Builder, structType string, o optionEntry) {
	optType, optOK := mapOptionType(o.GoType)
	b.WriteString("\t\t\t{\n")
	fmt.Fprintf(b, "\t\t\t\tName: %q,\n", o.YamlKey)
	if len(o.Aliases) > 0 {
		fmt.Fprintf(b, "\t\t\t\tAliases: []string{%s},\n", quotedList(o.Aliases))
	}
	if optOK {
		fmt.Fprintf(b, "\t\t\t\tType: registry.%s,\n", optType)
	} else {
		fmt.Fprintf(b, "\t\t\t\t// TODO(krit-gen): unsupported option go_type %q\n", o.GoType)
		fmt.Fprintf(b, "\t\t\t\tType: registry.OptString,\n")
	}
	fmt.Fprintf(b, "\t\t\t\tDefault: %s,\n", renderDefault(o))
	fmt.Fprintf(b, "\t\t\t\tDescription: %q,\n", o.Description)
	fmt.Fprintf(b, "\t\t\t\tApply: %s,\n", renderApply(structType, o, optType, optOK))
	b.WriteString("\t\t\t},\n")
}

// ----------------------------------------------------------------------
// Option encoding helpers.

func mapOptionType(goType string) (string, bool) {
	switch goType {
	case "int", "int-derived":
		return "OptInt", true
	case "bool":
		return "OptBool", true
	case "string":
		return "OptString", true
	case "[]string":
		return "OptStringList", true
	case "regex":
		return "OptRegex", true
	}
	return "", false
}

// renderDefault produces the Go literal syntax for the option default,
// matching the ConfigOption.Default interface{} field. For string-list
// defaults we emit a nil rather than empty slice to match the common
// "no default configured" convention.
func renderDefault(o optionEntry) string {
	switch o.GoType {
	case "int", "int-derived":
		switch v := o.Default.(type) {
		case float64:
			return strconv.Itoa(int(v))
		case int:
			return strconv.Itoa(v)
		case nil:
			return "0"
		default:
			return fmt.Sprintf("%v", v)
		}
	case "bool":
		if b, ok := o.Default.(bool); ok {
			return strconv.FormatBool(b)
		}
		return "false"
	case "string", "regex":
		if o.Default == nil {
			return `""`
		}
		if s, ok := o.Default.(string); ok {
			return strconv.Quote(s)
		}
		return `""`
	case "[]string":
		if o.Default == nil {
			return "[]string(nil)"
		}
		if items, ok := asStringSlice(o.Default); ok {
			if len(items) == 0 {
				return "[]string(nil)"
			}
			return "[]string{" + quotedList(items) + "}"
		}
		return "[]string(nil)"
	}
	// Unsupported — emit nil so the code still compiles.
	return "nil"
}

func asStringSlice(v interface{}) ([]string, bool) {
	switch s := v.(type) {
	case []string:
		return s, true
	case []interface{}:
		out := make([]string, 0, len(s))
		for _, el := range s {
			sv, ok := el.(string)
			if !ok {
				return nil, false
			}
			out = append(out, sv)
		}
		return out, true
	}
	return nil, false
}

func quotedList(items []string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, strconv.Quote(it))
	}
	return strings.Join(parts, ", ")
}

// renderApply produces the closure literal that downcasts and assigns
// the value for one option.
func renderApply(structType string, o optionEntry, optType string, optOK bool) string {
	if !optOK {
		return "nil"
	}
	var valueExpr string
	switch o.GoType {
	case "int", "int-derived":
		valueExpr = "value.(int)"
	case "bool":
		valueExpr = "value.(bool)"
	case "string":
		valueExpr = "value.(string)"
	case "[]string":
		valueExpr = "value.([]string)"
	case "regex":
		valueExpr = "value.(*regexp.Regexp)"
	default:
		return "nil"
	}
	return fmt.Sprintf(
		"func(target interface{}, value interface{}) { target.(*%s).%s = %s }",
		structType, o.Field, valueExpr,
	)
}

// ----------------------------------------------------------------------
// Oracle rendering.

// Oracle rendering removed — NeedsOracle now lives on v2.Rule.Oracle
// and v2.Capabilities, not on the registry descriptor. See
// roadmap/clusters/core-infra/oracle-filter-inversion.md.

// ----------------------------------------------------------------------
// Misc.

func formatFloat(f float64) string {
	if f == 0 {
		return "0"
	}
	// strconv.FormatFloat with -1 precision yields the shortest form
	// that round-trips, matching go/format idioms.
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func writeIfChanged(path string, content []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == string(content) {
		return nil
	}
	return os.WriteFile(path, content, 0o644)
}

// renderIndexFile emits the AllMetaProviders() slice plus a lazily-built
// metaByName map. Callers use this as the single source of truth for
// rule descriptors because the global Registry wraps some rules in
// adapters that hide the concrete struct pointer from Unwrap.
//
// The slice is keyed by struct type (deduplicated — the 4 alias
// registrations share a struct with their primary), sorted alphabetically
// for deterministic output.
func renderIndexFile(pkgName string, rules []ruleEntry) ([]byte, error) {
	// Collect unique struct types across the whole inventory, skipping
	// any struct whose Meta() lives in a hand-written file that still
	// counts as a MetaProvider (those are emitted — excludedStructs does
	// NOT mean "no Meta()", it means "generator skips the codegen path").
	// All hand-written Meta() files also return the descriptor, so they
	// belong in the index too.
	seen := map[string]bool{}
	for _, r := range rules {
		seen[r.StructType] = true
	}
	types := make([]string, 0, len(seen))
	for t := range seen {
		types = append(types, t)
	}
	sort.Strings(types)

	var b strings.Builder
	b.WriteString("// Code generated by krit-gen. DO NOT EDIT.\n")
	b.WriteString("// generator: internal/codegen/cmd/krit-gen\n")
	b.WriteString("// source: metaByName index derived from build/rule_inventory.json\n\n")
	fmt.Fprintf(&b, "package %s\n\n", pkgName)
	b.WriteString("import (\n")
	b.WriteString("\t\"sync\"\n\n")
	b.WriteString("\t\"github.com/kaeawc/krit/internal/rules/registry\"\n")
	b.WriteString(")\n\n")

	b.WriteString("// AllMetaProviders returns a zero-value pointer for every rule\n")
	b.WriteString("// struct that declares Meta(). The slice is used to build a name-\n")
	b.WriteString("// indexed metaByName map without depending on Registry entries\n")
	b.WriteString("// (which may be v2 adapter wrappers whose concrete struct is not\n")
	b.WriteString("// reachable via Unwrap).\n")
	b.WriteString("func AllMetaProviders() []registry.MetaProvider {\n")
	b.WriteString("\treturn []registry.MetaProvider{\n")
	for _, t := range types {
		fmt.Fprintf(&b, "\t\t(*%s)(nil),\n", t)
	}
	b.WriteString("\t}\n")
	b.WriteString("}\n\n")

	b.WriteString("var (\n")
	b.WriteString("\tmetaByNameOnce sync.Once\n")
	b.WriteString("\tmetaByNameMap  map[string]registry.RuleDescriptor\n")
	b.WriteString(")\n\n")

	b.WriteString("// metaByName builds a name-indexed map from AllMetaProviders.\n")
	b.WriteString("// Each provider's Meta() is called once on first use; the map is\n")
	b.WriteString("// keyed by RuleDescriptor.ID.\n")
	b.WriteString("func metaByName() map[string]registry.RuleDescriptor {\n")
	b.WriteString("\tmetaByNameOnce.Do(func() {\n")
	b.WriteString("\t\tproviders := AllMetaProviders()\n")
	b.WriteString("\t\tmetaByNameMap = make(map[string]registry.RuleDescriptor, len(providers))\n")
	b.WriteString("\t\tfor _, p := range providers {\n")
	b.WriteString("\t\t\tm := p.Meta()\n")
	b.WriteString("\t\t\tmetaByNameMap[m.ID] = m\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n")
	b.WriteString("\treturn metaByNameMap\n")
	b.WriteString("}\n")

	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("gofmt generated index: %w\n---\n%s", err, b.String())
	}
	return formatted, nil
}
