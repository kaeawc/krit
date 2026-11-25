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
	result *moshiResult
}

func analyzeMoshiProject(files []*scanner.File) *moshiResult {
	a := &moshiAnalyzer{result: &moshiResult{files: files, byRule: map[string][]moshiFinding{}}}
	for _, file := range files {
		if file == nil || file.FlatTree == nil || !moshiFileMayMatch(file.Content) {
			continue
		}
		a.analyzeFile(file)
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
		if decl, ok := moshiClassDecl(file, idx); ok && decl.isEnum {
			a.checkEnum(decl)
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
		case "object_declaration":
		default:
			return c, false
		}
		c.nameNode, _ = file.FlatFindChild(idx, "type_identifier")
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

	for i, k := range c.constants {
		if i == unknownIndex {
			continue
		}
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
