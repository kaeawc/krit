package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

var knownLoggerPackagePrefixes = []string{
	"java.util.logging.",
	"org.slf4j.",
	"ch.qos.logback.",
	"org.apache.logging.log4j.",
	"mu.",
	"io.github.oshai.kotlinlogging.",
}

// buildLoggerImportsFromAST returns whether any known logger package is
// imported by the file, plus a map of alias/simple-name to FQN for all
// known logger imports (used to resolve aliased logger receivers).
// Backed by the shared filefacts.Cache so the underlying import set is
// computed once per file per run.
func buildLoggerImportsFromAST(file *scanner.File) (bool, map[string]string) {
	if file == nil {
		return false, nil
	}
	imports := fileFactsCache().Imports(file)
	aliases := make(map[string]string)
	found := false
	for name, fqn := range imports.Aliases {
		for _, prefix := range knownLoggerPackagePrefixes {
			if strings.HasPrefix(fqn, prefix) {
				found = true
				aliases[name] = fqn
				break
			}
		}
	}
	for w := range imports.Wildcards {
		for _, prefix := range knownLoggerPackagePrefixes {
			if strings.HasPrefix(w, prefix) {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	return found, aliases
}

func receiverHasKnownLoggerTypeFlat(file *scanner.File, idx uint32, receiver string) bool {
	if file == nil || idx == 0 || receiver == "" {
		return false
	}

	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_declaration":
			if receiverHasKnownLoggerTypeInParametersFlat(file, parent, receiver) {
				return true
			}
		case "statements", "class_body", "source_file":
			if receiverHasKnownLoggerTypeInPropertiesFlat(file, parent, receiver) {
				return true
			}
		case "class_declaration":
			if receiverHasKnownLoggerTypeInClassParametersFlat(file, parent, receiver) {
				return true
			}
		}
	}

	return false
}

func receiverHasKnownLoggerTypeInParametersFlat(file *scanner.File, function uint32, receiver string) bool {
	params, _ := file.FlatFindChild(function, "function_value_parameters")
	if params == 0 {
		return false
	}

	for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
		if !file.FlatIsNamed(param) || file.FlatType(param) != "parameter" {
			continue
		}
		if extractIdentifierFlat(file, param) != receiver {
			continue
		}
		return isKnownLoggerTypeText(explicitTypeTextFlat(file, param))
	}

	return false
}

func receiverHasKnownLoggerTypeInPropertiesFlat(file *scanner.File, container uint32, receiver string) bool {
	for child := file.FlatFirstChild(container); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) || file.FlatType(child) != "property_declaration" {
			continue
		}
		if extractIdentifierFlat(file, child) != receiver {
			continue
		}
		return isKnownLoggerTypeText(explicitTypeTextFlat(file, child))
	}

	return false
}

func receiverHasKnownLoggerTypeInClassParametersFlat(file *scanner.File, classDecl uint32, receiver string) bool {
	ctor, _ := file.FlatFindChild(classDecl, "primary_constructor")
	if ctor == 0 {
		return false
	}

	for i := 0; i < file.FlatNamedChildCount(ctor); i++ {
		param := file.FlatNamedChild(ctor, i)
		if param == 0 || file.FlatType(param) != "class_parameter" {
			continue
		}
		if extractIdentifierFlat(file, param) != receiver || !classParameterDefinesPropertyFlat(file, param) {
			continue
		}
		return isKnownLoggerTypeText(explicitTypeTextFlat(file, param))
	}

	return false
}

func classParameterDefinesPropertyFlat(file *scanner.File, param uint32) bool {
	for c := file.FlatFirstChild(param); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "binding_pattern_kind" {
			return true
		}
	}
	return false
}

func explicitTypeTextFlat(file *scanner.File, node uint32) string {
	if file == nil || node == 0 {
		return ""
	}

	if text := directExplicitTypeTextFlat(file, node); text != "" {
		return text
	}

	if child, ok := file.FlatFindChild(node, "variable_declaration"); ok {
		return directExplicitTypeTextFlat(file, child)
	}

	return ""
}

func directExplicitTypeTextFlat(file *scanner.File, node uint32) string {
	colonSeen := false
	for i := 0; i < file.FlatChildCount(node); i++ {
		child := file.FlatChild(node, i)
		if child == 0 {
			continue
		}
		if file.FlatType(child) == ":" {
			colonSeen = true
			continue
		}
		if !colonSeen {
			continue
		}

		switch file.FlatType(child) {
		case "user_type", "nullable_type", "type_identifier", "function_type", "parenthesized_type":
			return file.FlatNodeText(child)
		}
	}

	return ""
}

// LoggerInterpolatedMessageRule detects SLF4J/Logback/log4j-style logger calls
// whose message argument is a Kotlin string template with interpolations.
// Parameterized logging (`logger.info("user {} logged in", id)`) is preferred
// because the template caches and the call skips argument evaluation when the
// level is disabled. Timber is excluded — its API is designed around Kotlin
// string interpolation. The lazy lambda form
// (`logger.info { "user $id logged in" }`) is also excluded because it defers
// evaluation until the level is enabled.
type LoggerInterpolatedMessageRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *LoggerInterpolatedMessageRule) Confidence() float64 { return api.ConfidenceMedium }

// loggerLevelMethods are the SLF4J-style log level method names that take a
// message template as the first argument.
var loggerLevelMethods = map[string]bool{
	"trace":   true,
	"debug":   true,
	"info":    true,
	"warn":    true,
	"warning": true,
	"error":   true,
}

// loggerConventionalReceivers are receiver identifiers that idiomatically
// refer to a logger instance even without an explicit type declaration.
var loggerConventionalReceivers = map[string]bool{
	"logger": true,
	"log":    true,
	"LOG":    true,
	"LOGGER": true,
}

// receiverIsKnownLoggerFlat reports whether the receiver of `call` is
// recognised as a logger instance. It checks the conventional names, declared
// types, and module-level imports — but never matches Timber, which is the
// rule's documented carve-out.
func receiverIsKnownLoggerFlat(file *scanner.File, call uint32, receiver string) bool {
	if receiver == "" || receiver == "Timber" {
		return false
	}
	if loggerConventionalReceivers[receiver] {
		return true
	}
	if receiverHasKnownLoggerTypeFlat(file, call, receiver) {
		return true
	}
	knownLoggerImport, aliases := buildLoggerImportsFromAST(file)
	if isLikelyLogReceiver(receiver, aliases) {
		return true
	}
	return knownLoggerImport
}

func receiverIsKnownParameterizedLoggerFlat(file *scanner.File, call uint32, receiver string) bool {
	if receiver == "" || receiver == "Timber" {
		return false
	}
	knownLoggerImport, aliases := buildLoggerImportsFromAST(file)
	if known, typed := receiverExplicitLoggerTypeStatusFlat(file, call, receiver, aliases); typed {
		return known
	}
	if receiverHasKnownLoggerTypeFlat(file, call, receiver) {
		return true
	}
	if isLikelyLogReceiver(receiver, aliases) {
		return true
	}
	if receiver == "logger" && receiverIsGradleTaskLoggerFlat(file, call) {
		return true
	}
	return loggerConventionalReceivers[receiver] && knownLoggerImport
}

func receiverExplicitLoggerTypeStatusFlat(file *scanner.File, idx uint32, receiver string, aliases map[string]string) (bool, bool) {
	if file == nil || idx == 0 || receiver == "" {
		return false, false
	}

	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_declaration":
			if known, typed := receiverExplicitLoggerTypeInParametersFlat(file, parent, receiver, aliases); typed {
				return known, true
			}
		case "statements", "class_body", "source_file":
			if known, typed := receiverExplicitLoggerTypeInPropertiesFlat(file, parent, receiver, aliases); typed {
				return known, true
			}
		case "class_declaration":
			if known, typed := receiverExplicitLoggerTypeInClassParametersFlat(file, parent, receiver, aliases); typed {
				return known, true
			}
		}
	}

	return false, false
}

func receiverExplicitLoggerTypeInParametersFlat(file *scanner.File, function uint32, receiver string, aliases map[string]string) (bool, bool) {
	params, _ := file.FlatFindChild(function, "function_value_parameters")
	if params == 0 {
		return false, false
	}

	for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
		if !file.FlatIsNamed(param) || file.FlatType(param) != "parameter" {
			continue
		}
		if extractIdentifierFlat(file, param) != receiver {
			continue
		}
		return loggerTypeTextStatusFlat(explicitTypeTextFlat(file, param), aliases)
	}

	return false, false
}

func receiverExplicitLoggerTypeInPropertiesFlat(file *scanner.File, container uint32, receiver string, aliases map[string]string) (bool, bool) {
	for child := file.FlatFirstChild(container); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) || file.FlatType(child) != "property_declaration" {
			continue
		}
		if extractIdentifierFlat(file, child) != receiver {
			continue
		}
		return loggerTypeTextStatusFlat(explicitTypeTextFlat(file, child), aliases)
	}

	return false, false
}

func receiverExplicitLoggerTypeInClassParametersFlat(file *scanner.File, classDecl uint32, receiver string, aliases map[string]string) (bool, bool) {
	ctor, _ := file.FlatFindChild(classDecl, "primary_constructor")
	if ctor == 0 {
		return false, false
	}

	for i := 0; i < file.FlatNamedChildCount(ctor); i++ {
		param := file.FlatNamedChild(ctor, i)
		if param == 0 || file.FlatType(param) != "class_parameter" {
			continue
		}
		if extractIdentifierFlat(file, param) != receiver || !classParameterDefinesPropertyFlat(file, param) {
			continue
		}
		return loggerTypeTextStatusFlat(explicitTypeTextFlat(file, param), aliases)
	}

	return false, false
}

func loggerTypeTextStatusFlat(text string, aliases map[string]string) (bool, bool) {
	text = compactConditionText(strings.TrimSuffix(strings.TrimSpace(text), "?"))
	if text == "" {
		return false, false
	}
	if isKnownLoggerTypeText(text) {
		return true, true
	}
	if fqn, ok := aliases[text]; ok && isKnownLoggerTypeText(fqn) {
		return true, true
	}
	return false, true
}

func receiverIsGradleTaskLoggerFlat(file *scanner.File, call uint32) bool {
	classDecl, ok := flatEnclosingAncestor(file, call, "class_declaration")
	return ok && classHasSupertypeNamed(file, classDecl, "DefaultTask")
}

// loggerInterpolatedMessageArgFlat returns the message argument of `call` when
// it is a positional or named string-template argument that contains
// interpolations. It deliberately ignores the lazy lambda form
// (`logger.info { ... }`) because that form is the recommended substitute.
func loggerInterpolatedMessageArgFlat(file *scanner.File, call uint32) uint32 {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return 0
	}

	if arg := flatNamedValueArgument(file, args, "message"); arg != 0 {
		if flatContainsStringInterpolation(file, arg) {
			return arg
		}
		return 0
	}

	first := flatPositionalValueArgument(file, args, 0)
	if first == 0 {
		return 0
	}
	if flatContainsStringInterpolation(file, first) {
		return first
	}
	return 0
}
