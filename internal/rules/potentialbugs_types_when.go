package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// ElseCaseInsteadOfExhaustiveWhenRule detects when with else on enum/sealed.
// With type inference: checks if ALL sealed/enum variants are covered in the
// when branches. If they are, the else is truly unnecessary. Without resolver,
// falls back to the heuristic (flags any when-with-else that uses `is` checks).
// ---------------------------------------------------------------------------
type ElseCaseInsteadOfExhaustiveWhenRule struct {
	FlatDispatchBase
	BaseRule
	// IgnoredSubjectTypes lists fully qualified subject types the rule skips, like detekt's option.
	IgnoredSubjectTypes []string
}

// Confidence reports a tier-2 (medium) base confidence — with the
// resolver it checks if sealed/enum variants are fully covered; fallback
// flags any when-with-else-using-is, which is noisier. Classified per
// roadmap/17.
func (r *ElseCaseInsteadOfExhaustiveWhenRule) Confidence() float64 { return api.ConfidenceMedium }

// whenHasElseBranchFlat returns true when the when_expression at idx has
// an `else ->` branch. A when_entry for the else branch has an `else`
// token child in place of a when_condition.
func whenHasElseBranchFlat(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatType(idx) != "when_expression" {
		return false
	}
	for entry := file.FlatFirstChild(idx); entry != 0; entry = file.FlatNextSib(entry) {
		if file.FlatType(entry) != "when_entry" {
			continue
		}
		for child := file.FlatFirstChild(entry); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "else" {
				return true
			}
		}
	}
	return false
}

func whenElseBranchTerminatesFlat(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatType(idx) != "when_expression" {
		return false
	}
	for entry := file.FlatFirstChild(idx); entry != 0; entry = file.FlatNextSib(entry) {
		if file.FlatType(entry) != "when_entry" {
			continue
		}
		isElse := false
		for child := file.FlatFirstChild(entry); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "else" {
				isElse = true
				break
			}
		}
		if !isElse {
			continue
		}
		if body, ok := file.FlatFindChild(entry, "control_structure_body"); ok {
			return blockTerminatesFlat(file, body)
		}
		return blockTerminatesFlat(file, entry)
	}
	return false
}

// whenSubjectExhaustiveKindFlat is the kind-aware variant of
// whenSubjectExhaustiveVariantsFlat: it returns whether the subject is a
// sealed hierarchy or an enum so callers can match variants by `is`
// type-test or by entry value. kind is "sealed" or "enum"; empty when
// the subject doesn't resolve to either.
func whenSubjectExhaustiveKindFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) (kind, subjectName string, variants []string) {
	if file == nil || idx == 0 || resolver == nil || file.FlatType(idx) != "when_expression" {
		return "", "", nil
	}
	subject := whenSubjectExpressionFlat(file, idx)
	if subject == 0 {
		return "", "", nil
	}
	candidates := whenSubjectTypeCandidatesFlat(file, subject, resolver)
	for _, name := range candidates {
		if vs := resolver.SealedVariants(name); len(vs) > 0 {
			return "sealed", simpleTypeName(name), vs
		}
		if vs := resolver.EnumEntries(name); len(vs) > 0 {
			return "enum", simpleTypeName(name), vs
		}
		if info := resolver.ClassHierarchy(name); info != nil {
			for _, infoName := range []string{info.Name, info.FQN} {
				if infoName == "" || infoName == name {
					continue
				}
				if vs := resolver.SealedVariants(infoName); len(vs) > 0 {
					return "sealed", simpleTypeName(infoName), vs
				}
				if vs := resolver.EnumEntries(infoName); len(vs) > 0 {
					return "enum", simpleTypeName(infoName), vs
				}
			}
		}
	}
	return "", "", nil
}

func whenSubjectExpressionFlat(file *scanner.File, idx uint32) uint32 {
	subject, ok := file.FlatFindChild(idx, "when_subject")
	if !ok || subject == 0 {
		return 0
	}
	for child := file.FlatFirstChild(subject); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "variable_declaration", "property_declaration":
			if init := declInitializerFlat(file, child); init != 0 {
				return init
			}
		default:
			return child
		}
	}
	return 0
}

func whenSubjectTypeCandidatesFlat(file *scanner.File, subject uint32, resolver typeinfer.TypeResolver) []string {
	seen := make(map[string]bool, 4)
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	addType := func(t *typeinfer.ResolvedType) {
		if t == nil || t.Kind == typeinfer.TypeUnknown {
			return
		}
		add(t.Name)
		add(t.FQN)
		if imported := resolver.ResolveImport(t.Name, file); imported != "" {
			add(imported)
		}
	}
	addType(resolver.ResolveFlatNode(subject, file))
	if file.FlatType(subject) == "simple_identifier" {
		name := file.FlatNodeText(subject)
		addType(resolver.ResolveByNameFlat(name, subject, file))
		if imported := resolver.ResolveImport(name, file); imported != "" {
			add(imported)
		}
	}
	return out
}

// whenUnguardedTypeTests returns the `is T` type names of entries that have no guard. A guarded
// entry (`is T if cond ->`) only matches some instances of T, so it does not cover T.
func whenUnguardedTypeTests(file *scanner.File, idx uint32) map[string]bool {
	covered := make(map[string]bool)
	file.FlatForEachChild(idx, func(entry uint32) {
		if file.FlatType(entry) != "when_entry" || whenEntryIsGuarded(file, entry) {
			return
		}
		file.FlatForEachChild(entry, func(cond uint32) {
			if file.FlatType(cond) != "when_condition" {
				return
			}
			if typeName := whenConditionTypeTestName(file, cond); typeName != "" {
				covered[typeName] = true
			}
		})
	})
	return covered
}

var whenGuardKeywordPattern = regexp.MustCompile(`(^|[\s)])if[\s(]`)

// whenEntryIsGuarded reports an entry whose condition part has an `if` guard, which the grammar
// parses as an error inside the condition.
func whenEntryIsGuarded(file *scanner.File, entry uint32) bool {
	for child := file.FlatFirstChild(entry); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "->" {
			prefix := file.Content[file.FlatStartByte(entry):file.FlatStartByte(child)]
			return whenGuardKeywordPattern.Match(prefix)
		}
	}
	return false
}

// whenSealedFullyCovered reports whether the covered type tests reach every permitted subtype of a
// sealed type, including the subtypes of nested sealed types.
func whenSealedFullyCovered(file *scanner.File, resolver typeinfer.TypeResolver, sealedName string, variants []string, covered map[string]bool, depth int) bool {
	counts := map[string]int{}
	whenCountHierarchyNames(resolver, sealedName, variants, counts, 0)
	return whenSealedCoveredAt(file, resolver, sealedName, variants, covered, counts, depth)
}

// whenCountHierarchyNames counts the simple names of every subtype below a sealed type, so a name
// declared in two nested types is known to be ambiguous.
func whenCountHierarchyNames(resolver typeinfer.TypeResolver, sealedName string, variants []string, counts map[string]int, depth int) {
	if depth > 8 {
		return
	}
	seen := map[string]bool{}
	all := append([]string(nil), variants...)
	all = append(all, resolver.SealedVariants(typeinfer.SealedIntermediateKey(sealedName))...)
	for _, v := range all {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		counts[simpleTypeName(v)]++
		if info := resolver.ClassHierarchy(v); info != nil && info.IsSealed {
			whenCountHierarchyNames(resolver, v, resolver.SealedVariants(v), counts, depth+1)
		}
	}
}

func whenSealedCoveredAt(file *scanner.File, resolver typeinfer.TypeResolver, sealedName string, variants []string, covered map[string]bool, counts map[string]int, depth int) bool {
	if depth > 8 {
		return false
	}
	all := append([]string(nil), variants...)
	all = append(all, resolver.SealedVariants(typeinfer.SealedIntermediateKey(sealedName))...)
	if len(all) == 0 {
		return false
	}
	for _, v := range all {
		if v == "" || whenTypeTestCovers(file, resolver, sealedName, covered, v, counts[simpleTypeName(v)] > 1) {
			continue
		}
		// A sealed variant is covered by covering each of its own subtypes.
		sub := resolver.SealedVariants(v)
		sub = append(sub, resolver.SealedVariants(typeinfer.SealedIntermediateKey(v))...)
		if info := resolver.ClassHierarchy(v); len(sub) > 0 && info != nil && info.IsSealed &&
			whenSealedCoveredAt(file, resolver, v, resolver.SealedVariants(v), covered, counts, depth+1) {
			continue
		}
		return false
	}
	return true
}

// whenTypeTestCovers matches a variant against the tested type names. A qualified test (`Outer.Inner`)
// only covers the variant whose resolved name ends with that qualified name, so a same-named class
// elsewhere does not count. An unqualified test of an ambiguous name covers the variant only when
// its import is declared in the variant's owner.
func whenTypeTestCovers(file *scanner.File, resolver typeinfer.TypeResolver, sealedName string, covered map[string]bool, variant string, ambiguous bool) bool {
	if covered[variant] && (!ambiguous || strings.Contains(variant, ".") || whenImportOwnerMatches(file, resolver, variant, variant, sealedName)) {
		return true
	}
	simple := simpleTypeName(variant)
	qualifiedVariant := variant
	if info := resolver.ClassHierarchy(variant); info != nil && info.FQN != "" {
		qualifiedVariant = info.FQN
	}
	for name := range covered {
		if simpleTypeName(name) != simple {
			continue
		}
		if ambiguous && !strings.Contains(name, ".") && !whenImportOwnerMatches(file, resolver, name, variant, sealedName) {
			continue
		}
		// Nested variants may resolve without their parent in the name, so the sealed type also qualifies them.
		qualifier := strings.TrimSuffix(strings.TrimSuffix(name, simple), ".")
		if !strings.Contains(name, ".") || qualifiedVariant == name || strings.HasSuffix(qualifiedVariant, "."+name) ||
			simpleTypeName(qualifier) == simpleTypeName(sealedName) {
			return true
		}
	}
	return false
}

// whenImportOwnerMatches reports whether an unqualified type test refers to a class nested in the
// variant's owner type. Tests with no nesting owner (top-level or unresolved) match any variant.
func whenImportOwnerMatches(file *scanner.File, resolver typeinfer.TypeResolver, testName, variant, sealedName string) bool {
	imported := whenExplicitImport(file, testName)
	if imported == "" {
		imported = resolver.ResolveImport(testName, file)
	}
	testOwner := nestingOwner(imported)
	if testOwner == "" {
		return true
	}
	variantOwner := nestingOwner(variant)
	if variantOwner == "" {
		variantOwner = simpleTypeName(sealedName)
	}
	return testOwner == variantOwner
}

var whenImportPattern = regexp.MustCompile(`(?m)^\s*import\s+([\w.]+)\s*$`)

// whenExplicitImport returns the fully qualified name an explicit import gives a simple name, or "".
// The file's own import lines are read because the resolver prefers a same-file declaration.
func whenExplicitImport(file *scanner.File, simple string) string {
	for _, m := range whenImportPattern.FindAllSubmatch(file.Content, -1) {
		if path := string(m[1]); simpleTypeName(path) == simple {
			return path
		}
	}
	return ""
}

// nestingOwner returns the enclosing class segment of a dotted name, or "" when the name has no
// class owner (package segments start in lower case).
func nestingOwner(qualified string) string {
	parts := strings.Split(qualified, ".")
	if len(parts) < 2 {
		return ""
	}
	owner := parts[len(parts)-2]
	if owner == "" || owner[0] < 'A' || owner[0] > 'Z' {
		return ""
	}
	return owner
}

// whenAddUnguardedObjectBranches adds value branches (`Loading ->`, `Outer.Loading ->`) that name an
// object, since an object is covered by equality as well as by `is`.
func whenAddUnguardedObjectBranches(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver, covered map[string]bool) {
	file.FlatForEachChild(idx, func(entry uint32) {
		if file.FlatType(entry) != "when_entry" || whenEntryIsGuarded(file, entry) {
			return
		}
		file.FlatForEachChild(entry, func(cond uint32) {
			name := whenConditionEnumEntryName(file, cond)
			if name == "" {
				return
			}
			if info := resolver.ClassHierarchy(name); info != nil && info.Kind == "object" {
				covered[name] = true
			}
		})
	})
}

// whenConditionTypeTestName returns the type identifier from an
// `is TypeName` when_condition, or "" if the condition is not a
// type_test. Works on the condition node, not the whole entry.
func whenConditionTypeTestName(file *scanner.File, condition uint32) string {
	if file == nil || file.FlatType(condition) != "when_condition" {
		return ""
	}
	typeTest, ok := file.FlatFindChild(condition, "type_test")
	if !ok {
		return ""
	}
	// `!is T` covers everything except T.
	if strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(typeTest)), "!is") {
		return ""
	}
	userType, ok := file.FlatFindChild(typeTest, "user_type")
	if !ok {
		return ""
	}
	return strings.TrimSpace(file.FlatNodeText(userType))
}

// ---------------------------------------------------------------------------
// NoElseInWhenSealedRule — the inverse of ElseCaseInsteadOfExhaustiveWhen.
// Flags `when` expressions whose subject is a sealed type or enum, that
// have no `else` branch, and that are missing one or more variants. Mimics
// kotlinc's NO_ELSE_IN_WHEN / NON_EXHAUSTIVE_WHEN_STATEMENT diagnostics for
// the cases the resolver can prove from source — sealed hierarchies and
// enums declared in the workspace.
//
// Without a resolver the rule cannot identify variants and stays silent.
// Library-defined sealed types (e.g. kotlin.Result) require classpath
// metadata; until that lands the rule simply doesn't fire on them.
// ---------------------------------------------------------------------------
type NoElseInWhenSealedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *NoElseInWhenSealedRule) Confidence() float64 { return api.ConfidenceHigher }

// whenConditionEnumEntryName extracts the entry name from a value-style
// when_condition like `Color.RED` (navigation_expression) or a bare
// `RED` (simple_identifier). Returns "" when the condition isn't a
// recognizable enum-entry match.
func whenConditionEnumEntryName(file *scanner.File, condition uint32) string {
	if file == nil || file.FlatType(condition) != "when_condition" {
		return ""
	}
	for child := file.FlatFirstChild(condition); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "navigation_expression":
			if name := flatNavigationExpressionLastIdentifier(file, child); name != "" {
				return name
			}
		case "simple_identifier":
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

func collectWhenCoveredVariants(file *scanner.File, idx uint32) (typeNames map[string]bool, entryNames map[string]bool) {
	typeNames = map[string]bool{}
	entryNames = map[string]bool{}
	file.FlatForEachChild(idx, func(entry uint32) {
		// A guarded entry only matches some instances, so it covers nothing.
		if file.FlatType(entry) != "when_entry" || whenEntryIsGuarded(file, entry) {
			return
		}
		file.FlatForEachChild(entry, func(cond uint32) {
			if file.FlatType(cond) != "when_condition" {
				return
			}
			if name := whenConditionTypeTestName(file, cond); name != "" {
				typeNames[name] = true
			}
			if name := whenConditionEnumEntryName(file, cond); name != "" {
				entryNames[name] = true
			}
		})
	})
	return typeNames, entryNames
}

func missingSealedVariants(covered map[string]bool, variants []string) []string {
	// Normalize covered names to also include their simple-name form so
	// `is Result.Loading` matches a variant indexed as `Loading`.
	coveredSet := make(map[string]bool, len(covered)*2)
	for name := range covered {
		coveredSet[name] = true
		coveredSet[simpleTypeName(name)] = true
	}
	seen := map[string]bool{}
	var missing []string
	for _, v := range variants {
		if v == "" {
			continue
		}
		simple := simpleTypeName(v)
		if seen[simple] {
			continue
		}
		seen[simple] = true
		if coveredSet[v] || coveredSet[simple] {
			continue
		}
		missing = append(missing, simple)
	}
	return missing
}

func missingEnumEntries(covered map[string]bool, entries []string) []string {
	seen := map[string]bool{}
	var missing []string
	for _, e := range entries {
		if e == "" {
			continue
		}
		if seen[e] {
			continue
		}
		seen[e] = true
		if covered[e] {
			continue
		}
		missing = append(missing, e)
	}
	return missing
}

// NonExhaustiveWhenRule flags `when` expressions used in a value-consuming
// position on a sealed/enum/Boolean subject when one or more branches are
// missing and there is no `else`. The statement form is left to
// NoElseInWhenSealed.
type NonExhaustiveWhenRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *NonExhaustiveWhenRule) Confidence() float64 { return api.ConfidenceHigher }

// whenIsUsedAsExpression walks parents through transparent wrappers
// (parenthesized_expression, annotated_expression) and inspects the
// enclosing context. Returns false for statement positions (statements,
// control_structure_body, block-bodied function_body) and true for value-
// consuming positions (initializers, return, arguments, operands, ...).
func whenIsUsedAsExpression(file *scanner.File, idx uint32) bool {
	cur := idx
	for {
		parent, ok := file.FlatParent(cur)
		if !ok {
			return false
		}
		switch file.FlatType(parent) {
		case "parenthesized_expression", "annotated_expression":
			cur = parent
			continue
		case "statements", "control_structure_body":
			return false
		case "function_body":
			// Expression body iff `=` precedes the when; block bodies start with `{`.
			for c := file.FlatFirstChild(parent); c != 0; c = file.FlatNextSib(c) {
				if c == cur {
					return false
				}
				switch file.FlatType(c) {
				case "=":
					return true
				case "{":
					return false
				}
			}
			return false
		case "property_declaration", "variable_declaration",
			"jump_expression", "value_argument", "value_arguments",
			"assignment", "directly_assignable_expression",
			"disjunction", "conjunction", "equality", "comparison",
			"additive_expression", "multiplicative_expression", "range_expression",
			"infix_expression", "as_expression", "elvis_expression",
			"prefix_expression", "postfix_unary_expression", "spread_expression",
			"check_expression",
			"indexing_suffix", "call_suffix", "navigation_suffix",
			"lambda_literal", "anonymous_function":
			return true
		default:
			return false
		}
	}
}

func whenSubjectResolvesToBoolean(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if resolver == nil {
		return false
	}
	subject := whenSubjectExpressionFlat(file, idx)
	if subject == 0 {
		return false
	}
	for _, name := range whenSubjectTypeCandidatesFlat(file, subject, resolver) {
		if name == "Boolean" || name == "kotlin.Boolean" {
			return true
		}
	}
	return false
}

func collectBooleanCoveredLiterals(file *scanner.File, idx uint32) (hasTrue, hasFalse bool) {
	file.FlatForEachChild(idx, func(entry uint32) {
		if file.FlatType(entry) != "when_entry" || whenEntryIsGuarded(file, entry) {
			return
		}
		file.FlatForEachChild(entry, func(cond uint32) {
			if file.FlatType(cond) != "when_condition" {
				return
			}
			for c := file.FlatFirstChild(cond); c != 0; c = file.FlatNextSib(c) {
				if file.FlatType(c) != "boolean_literal" {
					continue
				}
				switch strings.TrimSpace(file.FlatNodeText(c)) {
				case "true":
					hasTrue = true
				case "false":
					hasFalse = true
				}
			}
		})
	})
	return hasTrue, hasFalse
}

// missingWhenSealedOrEnumVariants resolves the subject's sealed/enum kind
// and returns the missing variant/entry names. Returns ("", "", nil) when
// the subject doesn't resolve to a sealed hierarchy or enum, or when all
// branches are covered.
func missingWhenSealedOrEnumVariants(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) (kind, subjectName string, missing []string) {
	kind, subjectName, variants := whenSubjectExhaustiveKindFlat(file, idx, resolver)
	if kind == "" || len(variants) == 0 {
		return "", "", nil
	}
	typeNames, entryNames := collectWhenCoveredVariants(file, idx)
	switch kind {
	case "sealed":
		// An object variant is also matched by value (`Loading ->`).
		for name := range entryNames {
			typeNames[name] = true
		}
		missing = missingSealedVariants(typeNames, variants)
	case "enum":
		missing = missingEnumEntries(entryNames, variants)
	}
	return kind, subjectName, missing
}

// whenSubjectTypeIsSealed reports whether the resolver knows the named type as a sealed class or interface.
func whenSubjectTypeIsSealed(resolver typeinfer.TypeResolver, name string) bool {
	info := resolver.ClassHierarchy(name)
	return info != nil && info.IsSealed
}

// whenSubjectIsNullableFlat reports whether the subject resolves to a nullable type.
func whenSubjectIsNullableFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	subject := whenSubjectExpressionFlat(file, idx)
	if subject == 0 {
		return false
	}
	if t := resolver.ResolveFlatNode(subject, file); t != nil && t.IsNullable() {
		return true
	}
	if file.FlatType(subject) == "simple_identifier" {
		t := resolver.ResolveByNameFlat(file.FlatNodeText(subject), subject, file)
		return t != nil && t.IsNullable()
	}
	return false
}

// whenHasNullBranchFlat reports whether some when_entry has a `null` condition.
func whenHasNullBranchFlat(file *scanner.File, idx uint32) bool {
	found := false
	file.FlatForEachChild(idx, func(entry uint32) {
		if file.FlatType(entry) != "when_entry" {
			return
		}
		file.FlatForEachChild(entry, func(cond uint32) {
			if file.FlatType(cond) == "when_condition" && strings.TrimSpace(file.FlatNodeText(cond)) == "null" {
				found = true
			}
		})
	})
	return found
}

// whenSubjectIsIgnored reports whether the subject type is one of the configured ignoredSubjectTypes.
func whenSubjectIsIgnored(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver, ignored []string) bool {
	if len(ignored) == 0 {
		return false
	}
	subject := whenSubjectExpressionFlat(file, idx)
	if subject == 0 {
		return false
	}
	for _, candidate := range whenSubjectTypeCandidatesFlat(file, subject, resolver) {
		for _, name := range ignored {
			if strings.TrimSpace(name) == candidate {
				return true
			}
		}
	}
	return false
}
