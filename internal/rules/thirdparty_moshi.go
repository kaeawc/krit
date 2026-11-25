package rules

// Ports of slack-lints' MoshiUsageDetector (slackhq/slack-lints 0.11.1,
// slack-lint-checks/src/main/java/slack/lint/MoshiUsageDetector.kt).
//
// Lint runs MoshiUsageDetector as one detector that reports many issue IDs.
// krit mirrors that: moshiAnalysisFor runs one pass over the project's parsed
// files, cached for the run, and every registered MoshiUsage* rule emits only
// the findings stamped with its own ID. The pass follows the detector's
// control flow, including its early returns, so an ID reports exactly where
// Lint would. Facts the source cannot prove (an annotation whose FQN is not
// imported, a non-literal annotation argument) make the pass abstain rather
// than guess.

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	moshiRuleSet = "slack-lint"

	fqcnMoshiJSONClass = "com.squareup.moshi.JsonClass"
	fqcnMoshiJSON      = "com.squareup.moshi.Json"

	slotMoshiUsage = "moshiUsage"
)

// moshiFinding is one finding of the shared pass, reported under rule.
type moshiFinding struct {
	rule string
	file *scanner.File
	node uint32
	msg  string
}

// moshiResult holds the shared pass's findings by rule ID. files keeps the
// analyzed slice alive so the address-based cache key stays unique for the run.
type moshiResult struct {
	files  []*scanner.File
	byRule map[string][]moshiFinding
}

// moshiAnalysisFor returns the run's MoshiUsage findings, computing them once
// for all MoshiUsage* rules.
func moshiAnalysisFor(ctx *api.Context) *moshiResult {
	files := ctx.ParsedFiles
	if len(files) == 0 {
		return &moshiResult{}
	}
	key := fmt.Sprintf("%p/%d", &files[0], len(files))
	return filefacts.StringFact(fileFactsCache(), key, slotMoshiUsage, func() *moshiResult {
		return analyzeMoshiProject(files)
	})
}

// emitMoshiFindings reports the shared pass's findings for the rule ctx runs.
func emitMoshiFindings(ctx *api.Context) {
	if ctx.Rule == nil {
		return
	}
	for _, f := range moshiAnalysisFor(ctx).byRule[ctx.Rule.ID] {
		ctx.Emit(scanner.Finding{
			File:      f.file.Path,
			Line:      f.file.FlatRow(f.node) + 1,
			Col:       f.file.FlatCol(f.node) + 1,
			StartByte: int(f.file.FlatStartByte(f.node)),
			EndByte:   int(f.file.FlatEndByte(f.node)),
			Message:   f.msg,
		})
	}
}

// moshiAnalyzer accumulates findings over one project.
type moshiAnalyzer struct {
	result  *moshiResult
	sealed  *moshiSealedIndex
	classes *moshiClassIndex // built on first use by projectClasses
}

func analyzeMoshiProject(files []*scanner.File) *moshiResult {
	a := &moshiAnalyzer{result: &moshiResult{files: files, byRule: map[string][]moshiFinding{}}, sealed: buildMoshiSealedIndex(files)}
	for _, file := range files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		switch {
		case moshiFileMayMatch(file.Content):
			a.analyzeFile(file)
		case file.Language == scanner.LangKotlin && a.sealed.packages[moshiFilePackage(file)]:
			// A subtype of a moshi-sealed type needs @TypeLabel even when its
			// file never mentions Moshi.
			a.analyzeSealedSubtypes(file)
		}
	}
	return a.result
}

// moshiFileMayMatch is a cheap prefilter: every check needs a Moshi or
// moshix annotation, and naming one requires its package in the source.
func moshiFileMayMatch(content []byte) bool {
	return bytes.Contains(content, []byte("com.squareup.moshi")) || bytes.Contains(content, []byte("dev.zacsweers.moshix"))
}

func (a *moshiAnalyzer) report(rule string, file *scanner.File, node uint32, msg string) {
	if node == 0 {
		return
	}
	a.result.byRule[rule] = append(a.result.byRule[rule], moshiFinding{rule: rule, file: file, node: node, msg: msg})
}

func (a *moshiAnalyzer) reportIssue(rule string, file *scanner.File, node uint32) {
	a.report(rule, file, node, moshiIssueMessage(rule))
}

func (a *moshiAnalyzer) analyzeFile(file *scanner.File) {
	file.FlatWalkAllNodes(0, func(idx uint32) {
		decl, ok := moshiClassDecl(file, idx)
		if !ok {
			return
		}
		// Enums get checked in both languages; the remaining class checks are Kotlin-only.
		if decl.isEnum {
			a.checkEnum(decl)
			return
		}
		if moshiIsAnnotationType(decl) {
			a.checkJSONQualifier(decl)
		}
		if file.Language == scanner.LangKotlin {
			a.checkKotlinClass(decl)
		}
	})
}

// analyzeSealedSubtypes runs only the sealed-subtype checks, for a Kotlin file
// that does not mention Moshi but shares a package with a moshi-sealed type.
func (a *moshiAnalyzer) analyzeSealedSubtypes(file *scanner.File) {
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if decl, ok := moshiClassDecl(file, idx); ok && !decl.isEnum {
			a.checkSealedClass(decl)
		}
	})
}

// moshiClass is the language-neutral view of a class declaration.
type moshiClass struct {
	file      *scanner.File
	node      uint32
	nameNode  uint32
	isEnum    bool
	anns      []moshiAnnotation
	constants []moshiEnumConstant

	// Kotlin-only facts.
	isObject    bool
	isInterface bool
	visibility  uint32 // visibility_modifier node, or 0 for the public default
	primary     uint32 // primary_constructor node, or 0
	typeParams  uint32 // type_parameters node, or 0
}

// moshiEnumConstant is one enum entry (Kotlin) or enum constant (Java).
type moshiEnumConstant struct {
	node     uint32
	nameNode uint32
	name     string
	anns     []moshiAnnotation
}

// moshiClassDecl reads the class declared at idx, if idx is a class-like
// declaration in Kotlin or Java.
func moshiClassDecl(file *scanner.File, idx uint32) (moshiClass, bool) {
	c := moshiClass{file: file, node: idx}
	switch file.Language {
	case scanner.LangJava:
		switch file.FlatType(idx) {
		case "enum_declaration":
			c.isEnum = true
		case "class_declaration", "interface_declaration", "annotation_type_declaration", "record_declaration":
		default:
			return c, false
		}
		c.nameNode, _ = file.FlatFindChild(idx, "identifier")
	default:
		switch file.FlatType(idx) {
		case "class_declaration":
			c.isEnum = file.FlatHasChildOfType(idx, "enum")
		case "object_declaration", "companion_object":
			c.isObject = true
		default:
			return c, false
		}
		c.nameNode, _ = file.FlatFindChild(idx, "type_identifier")
		if c.nameNode == 0 && c.isObject {
			// An unnamed companion object: report at its `object` keyword.
			c.nameNode, _ = file.FlatFindChild(idx, "object")
		}
		c.isInterface = file.FlatHasChildOfType(idx, "interface")
		if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
			c.visibility, _ = file.FlatFindChild(mods, "visibility_modifier")
		}
		c.primary, _ = file.FlatFindChild(idx, "primary_constructor")
		c.typeParams, _ = file.FlatFindChild(idx, "type_parameters")
	}
	if c.nameNode == 0 {
		return c, false
	}
	c.anns = moshiAnnotationsOf(file, idx)
	if c.isEnum {
		c.constants = moshiEnumConstants(file, idx)
	}
	return c, true
}

func moshiEnumConstants(file *scanner.File, idx uint32) []moshiEnumConstant {
	bodyType, entryType, nameType := "enum_class_body", "enum_entry", "simple_identifier"
	if file.Language == scanner.LangJava {
		bodyType, entryType, nameType = "enum_body", "enum_constant", "identifier"
	}
	body, ok := file.FlatFindChild(idx, bodyType)
	if !ok {
		return nil
	}
	var out []moshiEnumConstant
	file.FlatForEachChild(body, func(entry uint32) {
		if file.FlatType(entry) != entryType {
			return
		}
		name, ok := file.FlatFindChild(entry, nameType)
		if !ok {
			return
		}
		out = append(out, moshiEnumConstant{
			node:     entry,
			nameNode: name,
			name:     file.FlatNodeText(name),
			anns:     moshiAnnotationsOf(file, entry),
		})
	})
	return out
}

// checkEnum ports MoshiUsageDetector.checkEnum. Enums are checked in Kotlin
// and Java.
func (a *moshiAnalyzer) checkEnum(c moshiClass) {
	file := c.file
	jsonClass := moshiFindAnnotation(file, c.anns, fqcnMoshiJSONClass)
	hasJSONConstant := false
	for _, k := range c.constants {
		if moshiFindAnnotation(file, k.anns, fqcnMoshiJSON) != nil {
			hasJSONConstant = true
			break
		}
	}
	// Not annotated and no member has @Json: not a Moshi enum.
	if jsonClass == nil && !hasJSONConstant {
		return
	}

	if jsonClass != nil {
		// A non-blank generator means a custom generator owns this enum, and
		// the detector skips the remaining checks.
		generator, known := moshiAnnotationStringArg(file, *jsonClass, "generator", 1, "")
		if !known {
			return
		}
		if strings.TrimSpace(generator) != "" {
			return
		}
		// Lint casts generateAdapter to a literal; anything else is unknown here.
		value := moshiAnnotationArg(file, *jsonClass, "generateAdapter", 0)
		b, ok := moshiBooleanLiteral(file, value)
		if !ok {
			return
		}
		if b {
			a.reportIssue("MoshiUsageEnumJsonClassGenerated", file, value)
		}
	} else {
		a.reportIssue("MoshiUsageEnumMissingJsonClass", file, c.nameNode)
	}

	unknownIndex := -1
	for i, k := range c.constants {
		if k.name == "UNKNOWN" {
			unknownIndex = i
			break
		}
	}
	switch {
	case unknownIndex == -1:
		a.reportIssue("MoshiUsageEnumMissingUnknown", file, c.nameNode)
	case unknownIndex != 0:
		a.reportIssue("MoshiUsageEnumMissingUnknown", file, c.constants[unknownIndex].nameNode)
	case moshiFindAnnotation(file, c.constants[0].anns, fqcnMoshiJSON) != nil:
		a.reportIssue("MoshiUsageEnumAnnotatedUnknown", file, c.nameNode)
	}

	jsonNames := map[string]moshiMember{}
	for i, k := range c.constants {
		if i == unknownIndex {
			continue
		}
		member := moshiMember{name: k.name, nameNode: k.nameNode, anns: k.anns}
		a.validateJSONName(file, member, moshiFindAnnotation(file, k.anns, fqcnMoshiJSON), jsonNames)
		if k.name != moshiScreamingSnakeCase(k.name) {
			a.reportIssue("MoshiUsageEnumCasing", file, k.nameNode)
		}
	}
}

// moshiScreamingSnakeCase ports slack-lints' String.toScreamingSnakeCase.
func moshiScreamingSnakeCase(s string) string {
	var b strings.Builder
	prevWasLower, pendingUnderscore, letterSeen := false, false, false
	for _, c := range s {
		switch c {
		case '_', '-', '.', ':', '/':
			if letterSeen {
				pendingUnderscore = true
			}
		default:
			letterSeen = true
			if pendingUnderscore {
				b.WriteByte('_')
			}
			pendingUnderscore = false
			if unicode.IsUpper(c) {
				if prevWasLower {
					prevWasLower = false
					b.WriteByte('_')
				}
				b.WriteRune(c)
			} else {
				prevWasLower = true
				b.WriteRune(unicode.ToUpper(c))
			}
		}
	}
	return b.String()
}

const fqcnMoshixAdaptedBy = "dev.zacsweers.moshix.adapters.AdaptedBy"

// moshiHasModifier reports whether the Kotlin declaration has the soft-keyword
// modifier (data, sealed, abstract, inner, value, inline, annotation, ...).
func moshiHasModifier(c moshiClass, modifier string) bool {
	return c.file.FlatHasModifier(c.node, modifier)
}

// moshiVisibilityAllowed reports whether a Kotlin visibility modifier node is
// one Moshi can reach: absent (public), public, or internal.
func moshiVisibilityAllowed(file *scanner.File, visibility uint32) bool {
	if visibility == 0 {
		return true
	}
	text := strings.TrimSpace(file.FlatNodeText(visibility))
	return text == "public" || text == "internal"
}

// checkKotlinClass ports the Kotlin class path of MoshiUsageDetector.visitClass.
func (a *moshiAnalyzer) checkKotlinClass(c moshiClass) {
	file := c.file
	adaptedBy := moshiFindAnnotation(file, c.anns, fqcnMoshixAdaptedBy)
	jsonClass := moshiFindAnnotation(file, c.anns, fqcnMoshiJSONClass)
	if adaptedBy != nil {
		if jsonClass != nil {
			a.reportIssue("MoshiUsageDoubleClassAnnotation", file, adaptedBy.node)
			a.reportIssue("MoshiUsageDoubleClassAnnotation", file, jsonClass.node)
		}
		a.validateAdaptedBy(file, c.node, *adaptedBy)
		return
	}

	a.checkSealedClass(c)

	if jsonClass == nil {
		return
	}

	if !moshiVisibilityAllowed(file, c.visibility) {
		a.reportIssue("MoshiUsageClassVisibility", file, c.visibility)
	}

	// Only a declared generator counts. A blank one is an error; any other value
	// means a custom generator owns the type.
	usesCustomGenerator := false
	if expr := moshiAnnotationArg(file, *jsonClass, "generator", 1); expr != 0 {
		generator, ok := moshiStringLiteral(file, expr)
		if !ok {
			return
		}
		if strings.TrimSpace(generator) == "" {
			a.reportIssue("MoshiUsageBlankGenerator", file, expr)
		} else {
			usesCustomGenerator = true
			if label, ok := strings.CutPrefix(generator, "sealed:"); ok {
				if strings.TrimSpace(label) == "" {
					a.reportIssue("MoshiUsageBlankTypeLabel", file, expr)
				}
				if !moshiHasModifier(c, "sealed") {
					a.reportIssue("MoshiUsageSealedMustBeSealed", file, c.nameNode)
				}
			}
		}
	}

	generateAdapterExpr := moshiAnnotationArg(file, *jsonClass, "generateAdapter", 0)
	generateAdapter, ok := moshiBooleanLiteral(file, generateAdapterExpr)
	if !ok {
		return
	}
	isData := moshiHasModifier(c, "data")
	if !generateAdapter {
		// generateAdapter = false is fine (it may be there for R8), except on a
		// data class, where it is probably a mistake.
		if isData {
			a.reportIssue("MoshiUsageGenerateAdapterShouldBeTrue", file, generateAdapterExpr)
		}
		return
	}

	if usesCustomGenerator {
		return
	}

	if c.isObject {
		a.reportIssue("MoshiUsageObject", file, jsonClass.node)
		return
	}
	// Lint's abstract check includes sealed types.
	if moshiHasModifier(c, "annotation") || c.isInterface || moshiHasModifier(c, "inner") ||
		moshiHasModifier(c, "abstract") || moshiHasModifier(c, "sealed") {
		a.reportIssue("MoshiUsageUnsupportedType", file, jsonClass.node)
		return
	}

	if !isData {
		a.reportIssue("MoshiUsageUseData", file, c.nameNode)
	}

	// UAST does not model a value (or inline) class's constructor as a
	// KtPrimaryConstructor, so Lint reports those as missing one too.
	if c.primary == 0 || moshiHasModifier(c, "value") || moshiHasModifier(c, "inline") {
		a.reportIssue("MoshiUsageMissingPrimary", file, c.nameNode)
		return
	}

	a.checkPrimaryConstructor(c)
}

const fqcnKotlinJvmTransient = "kotlin.jvm.Transient"

// moshiParam is one Kotlin primary-constructor parameter.
type moshiParam struct {
	node       uint32
	nameNode   uint32
	name       string
	anns       []moshiAnnotation
	binding    uint32 // the val/var binding_pattern_kind, or 0 for a plain parameter
	isVar      bool
	visibility uint32
	typeNode   uint32
	defaultVal uint32 // default value expression, or 0
}

func moshiParams(file *scanner.File, primary uint32) []moshiParam {
	var out []moshiParam
	file.FlatForEachChild(primary, func(child uint32) {
		if file.FlatType(child) != "class_parameter" {
			return
		}
		p := moshiParam{node: child, anns: moshiAnnotationsOf(file, child)}
		afterEquals := false
		file.FlatForEachChild(child, func(part uint32) {
			switch t := file.FlatType(part); {
			case t == "modifiers":
				p.visibility, _ = file.FlatFindChild(part, "visibility_modifier")
			case t == "binding_pattern_kind":
				p.binding = part
				p.isVar = file.FlatHasChildOfType(part, "var")
			case t == "simple_identifier" && p.nameNode == 0:
				p.nameNode = part
				p.name = file.FlatNodeText(part)
			case t == "=":
				afterEquals = true
			case afterEquals && p.defaultVal == 0:
				// The default may be an anonymous token such as `null`.
				p.defaultVal = part
			case !file.FlatIsNamed(part):
			case p.nameNode != 0 && p.typeNode == 0:
				p.typeNode = part
			}
		})
		if p.nameNode != 0 {
			out = append(out, p)
		}
	})
	return out
}

// moshiIsKotlinTransient reports whether ann is kotlin.jvm.Transient. A bare
// @Transient resolves through Kotlin's default kotlin.jvm import unless an
// import, a same-file class, or a star import of another package may shadow it.
func moshiIsKotlinTransient(file *scanner.File, ann moshiAnnotation) bool {
	if moshiRefIs(file, ann.name, fqcnKotlinJvmTransient) {
		return true
	}
	if ann.name != "Transient" {
		return false
	}
	imports := lintImportsOf(file)
	if _, ok := imports.bySimple["Transient"]; ok || lintSameFileClass(file, "Transient") != 0 {
		return false
	}
	for _, star := range imports.stars {
		if star != "com.squareup.moshi" && !strings.HasPrefix(star, "dev.zacsweers.moshix") && star != "kotlin.jvm" {
			return false
		}
	}
	return true
}

// checkPrimaryConstructor ports the primary-constructor part of
// MoshiUsageDetector.visitClass.
func (a *moshiAnalyzer) checkPrimaryConstructor(c moshiClass) {
	file := c.file
	if mods, ok := file.FlatFindChild(c.primary, "modifiers"); ok {
		if vis, ok := file.FlatFindChild(mods, "visibility_modifier"); ok && !moshiVisibilityAllowed(file, vis) {
			a.reportIssue("MoshiUsagePrivateConstructor", file, vis)
		}
	}
	jsonNames := map[string]moshiMember{}
	scope := moshiTypeScope{a: a, file: file, pkg: moshiFilePackage(file), class: c.node, typeParams: moshiTypeParamNames(file, c.node)}
	if mods, ok := file.FlatFindChild(c.node, "modifiers"); ok && moshiMentionsWildcardControl(file.FlatNodeText(mods)) {
		scope.wildcardsOverridden = true
	}
	if moshiMentionsWildcardControl(moshiFileAnnotationsText(file)) {
		scope.wildcardsOverridden = true
	}
	for _, p := range moshiParams(file, c.primary) {
		hasDefault := p.defaultVal != 0
		for _, ann := range p.anns {
			if moshiIsKotlinTransient(file, ann) && !hasDefault {
				a.reportIssue("MoshiUsageTransientNeedsInit", file, p.node)
				break
			}
		}
		if p.binding == 0 {
			if !hasDefault {
				a.reportIssue("MoshiUsageParamNeedsInit", file, p.node)
			}
			continue
		}
		if p.isVar {
			a.reportIssue("MoshiUsageVarProperty", file, p.binding)
		}
		if !moshiVisibilityAllowed(file, p.visibility) {
			a.reportIssue("MoshiUsagePrivateConstructorProperty", file, p.visibility)
		}
		adaptedBy := moshiFindAnnotation(file, p.anns, fqcnMoshixAdaptedBy)
		if adaptedBy != nil {
			a.validateAdaptedBy(file, c.node, *adaptedBy)
		}
		// An adapter or a JsonQualifier takes over the property's type.
		if adaptedBy == nil && p.typeNode != 0 && !scope.moshiPropertyHasQualifier(p.anns) {
			propScope := scope
			propScope.wildcardsOverridden = scope.wildcardsOverridden || moshiMentionsWildcardControl(file.FlatNodeText(p.node))
			propScope.checkPropertyType(p.typeNode, p.typeNode, p.defaultVal, false)
		}
		a.checkPropertyJSONName(file, p, jsonNames)
	}
}

const fqcnGsonSerializedName = "com.google.gson.annotations.SerializedName"

// moshiMember is an enum constant or constructor property whose JSON name
// MoshiUsageDetector.validateJsonName checks.
type moshiMember struct {
	name     string
	nameNode uint32
	anns     []moshiAnnotation
}

// validateJSONName ports MoshiUsageDetector.validateJsonName. seen maps each
// JSON name to the member that claimed it.
func (a *moshiAnalyzer) validateJSONName(file *scanner.File, member moshiMember, json *moshiAnnotation, seen map[string]moshiMember) {
	jsonName := member.name
	known := true
	if json != nil {
		// Without a name argument Moshi uses the member name. (Lint reads the
		// annotation's "\u0000" UNSET_NAME default there; see the fixtures.)
		if expr := moshiAnnotationArg(file, *json, "name", 0); expr != 0 {
			value, ok := moshiStringLiteral(file, expr)
			switch {
			case !ok:
				known = false
			case strings.TrimSpace(value) == "":
				a.reportIssue("MoshiUsageBlankJsonName", file, expr)
			case value == member.name:
				a.reportIssue("MoshiUsageRedundantJsonName", file, expr)
			default:
				jsonName = value
			}
		}
	}

	if known {
		if existing, dup := seen[jsonName]; dup {
			a.report("MoshiUsageDuplicateJsonName", file, member.nameNode,
				fmt.Sprintf("Name '%s' is duplicated by member '%s'.", jsonName, existing.name))
			a.report("MoshiUsageDuplicateJsonName", file, existing.nameNode,
				fmt.Sprintf("Name '%s' is duplicated by member '%s'.", jsonName, member.name))
		}
		seen[jsonName] = member
	}

	if serialized := moshiFindAnnotation(file, member.anns, fqcnGsonSerializedName); serialized != nil {
		a.reportIssue("MoshiUsageSerializedName", file, serialized.node)
	}
}

// checkPropertyJSONName ports the @Json handling of a constructor property:
// redundant site targets, the JSON name, and snake_case names.
func (a *moshiAnalyzer) checkPropertyJSONName(file *scanner.File, p moshiParam, seen map[string]moshiMember) {
	var jsonAnns []moshiAnnotation
	for _, ann := range p.anns {
		if moshiRefIs(file, ann.name, fqcnMoshiJSON) {
			jsonAnns = append(jsonAnns, ann)
		}
	}
	var toValidate *moshiAnnotation
	switch {
	case len(jsonAnns) > 1:
		// Keep the target-less one (or the first) and flag the others.
		keep := 0
		for i, ann := range jsonAnns {
			if ann.site == 0 {
				keep = i
				break
			}
		}
		for i, ann := range jsonAnns {
			if i != keep {
				a.reportIssue("MoshiUsageRedundantSiteTarget", file, ann.node)
			}
		}
	case len(jsonAnns) == 1:
		if jsonAnns[0].site != 0 {
			a.reportIssue("MoshiUsageRedundantSiteTarget", file, jsonAnns[0].site)
		}
		toValidate = &jsonAnns[0]
	}

	a.validateJSONName(file, moshiMember{name: p.name, nameNode: p.nameNode, anns: p.anns}, toValidate, seen)

	// With an @Json present the IDE already offers better renaming, so only
	// unannotated snake_case names are flagged.
	if toValidate == nil && p.name != moshiSnakeToCamel(p.name) {
		a.reportIssue("MoshiUsageSnakeCase", file, p.nameNode)
	}
}

// moshiSnakeToCamel ports slack-lints' String.snakeToCamel.
func moshiSnakeToCamel(s string) string {
	var b strings.Builder
	capNext, letterSeen := false, false
	for _, c := range s {
		if c == '_' || c == '-' {
			capNext = letterSeen
			continue
		}
		letterSeen = true
		if capNext {
			b.WriteRune(unicode.ToUpper(c))
			capNext = false
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}
