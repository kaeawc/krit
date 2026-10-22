package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// CouldBeSequenceRule detects collection operation chains that could be sequences.
type CouldBeSequenceRule struct {
	FlatDispatchBase
	BaseRule
	AllowedOperations int
}

// Confidence reports a tier-2 (medium) base confidence — detects long
// collection chains that should use asSequence(); chain-length heuristic is
// conservative but the threshold is a style call. Classified per
// roadmap/17.
func (r *CouldBeSequenceRule) Confidence() float64 { return api.ConfidenceMedium }

// collectionTypes that benefit from asSequence() conversion.
var sequenceCandidateTypes = map[string]bool{
	"List": true, "MutableList": true, "Collection": true,
	"Iterable": true, "Set": true, "MutableSet": true,
	"ArrayList": true, "HashSet": true, "LinkedHashSet": true,
}

// Types that should NOT be suggested for asSequence().
var sequenceExcludedTypes = map[string]bool{
	"Sequence": true, "Flow": true, "StateFlow": true,
	"SharedFlow": true, "MutableStateFlow": true, "MutableSharedFlow": true,
	"Observable": true, "Flowable": true, "Single": true, "Maybe": true,
	"Completable": true, "Flux": true, "Mono": true, "LiveData": true,
}

var sequenceReturnTypes = map[string]bool{"Sequence": true}

var obviousSequenceSourceCalls = map[string]bool{
	"sequenceOf":       true,
	"generateSequence": true,
	"emptySequence":    true,
	"asSequence":       true,
}

var obviousCollectionSourceCalls = map[string]bool{
	"listOf":        true,
	"mutableListOf": true,
	"arrayListOf":   true,
	"setOf":         true,
	"mutableSetOf":  true,
	"emptyList":     true,
	"emptySet":      true,
	"buildList":     true,
	"buildSet":      true,
}

var collectionAnnotationTypeNames = []string{
	"List", "MutableList",
	"Set", "MutableSet",
	"Collection", "Iterable",
}

var collectionOps = map[string]bool{
	"map": true, "filter": true, "flatMap": true, "sorted": true,
	"sortedBy": true, "sortedWith": true, "sortedDescending": true,
	"sortedByDescending": true, "reversed": true, "distinct": true,
	"distinctBy": true, "drop": true, "dropWhile": true,
	"take": true, "takeWhile": true, "zip": true,
}

type oracleLookupProvider interface {
	Oracle() oracle.Lookup
}

func sequenceCollectionOperationNames() []string {
	return []string{
		"distinct", "distinctBy", "drop", "dropWhile", "filter", "flatMap",
		"map", "reversed", "sorted", "sortedBy", "sortedByDescending",
		"sortedWith", "take", "takeWhile", "zip",
	}
}

func collectCollectionChainCallsFlat(file *scanner.File, idx uint32) []uint32 {
	var calls []uint32
	current := idx
	for current != 0 {
		name := flatCallExpressionName(file, current)
		if !collectionOps[name] {
			break
		}
		calls = append(calls, current)
		if parent, ok := file.FlatParent(current); ok && file.FlatType(parent) == "navigation_expression" {
			if gp, ok := file.FlatParent(parent); ok && file.FlatType(gp) == "call_expression" {
				current = gp
				continue
			}
		}
		break
	}
	return calls
}

func collectionChainShouldSkipStartFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return true
	}
	receiver := collectionCallReceiverFlat(file, idx)
	if receiver != 0 && collectionOps[flatCallExpressionName(file, receiver)] {
		return true
	}
	return collectionChainHasReceiverCallNamedFlat(file, idx, obviousSequenceSourceCalls)
}

func collectionCallReceiverFlat(file *scanner.File, call uint32) uint32 {
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return 0
	}
	receiver := flatUnwrapParenExpr(file, file.FlatNamedChild(navExpr, 0))
	if file.FlatType(receiver) == "call_expression" {
		return receiver
	}
	return 0
}

func collectionChainHasReceiverCallNamedFlat(file *scanner.File, idx uint32, names map[string]bool) bool {
	for current := idx; current != 0 && file.FlatType(current) == "call_expression"; {
		receiver := collectionCallReceiverFlat(file, current)
		if receiver == 0 {
			return false
		}
		if names[flatCallExpressionName(file, receiver)] {
			return true
		}
		current = receiver
	}
	return false
}

func sequenceOperationReturnsSequence(name string) bool {
	method := typeinfer.LookupStdlibMethod("Sequence", name)
	return method != nil && resolvedTypeMatches(method.ReturnType, sequenceReturnTypes)
}

func sequenceOracleDecisionFlat(file *scanner.File, resolver typeinfer.TypeResolver, calls []uint32) (decided bool, report bool) {
	provider, ok := resolver.(oracleLookupProvider)
	if !ok || provider.Oracle() == nil || file == nil {
		return false, false
	}
	for _, call := range calls {
		name := flatCallExpressionName(file, call)
		target := oracleLookupCallTargetFlat(provider.Oracle(), file, call)
		if target == "" || target == name {
			return false, false
		}
		if !sequenceOperationReturnsSequence(name) {
			return true, false
		}
		if !sequenceCallTargetMatchesName(target, name) {
			if sequenceCallTargetIsAnyKotlinCollectionOperation(target) {
				return false, false
			}
			return true, false
		}
		if !sequenceCallTargetIsKotlinCollection(target) {
			return true, false
		}
	}
	return len(calls) > 0, true
}

func sequenceCallTargetMatchesName(target, name string) bool {
	return target == "kotlin.collections."+name ||
		strings.HasSuffix(target, "."+name) ||
		strings.HasSuffix(target, "#"+name)
}

func sequenceCallTargetIsKotlinCollection(target string) bool {
	return strings.HasPrefix(target, "kotlin.collections.")
}

func sequenceCallTargetIsAnyKotlinCollectionOperation(target string) bool {
	if !sequenceCallTargetIsKotlinCollection(target) {
		return false
	}
	for _, name := range sequenceCollectionOperationNames() {
		if sequenceCallTargetMatchesName(target, name) {
			return true
		}
	}
	return false
}

func sequenceResolverDecisionFlat(file *scanner.File, resolver typeinfer.TypeResolver, rootReceiver uint32, calls []uint32) (decided bool, report bool) {
	if resolver == nil || file == nil || rootReceiver == 0 {
		return false, false
	}
	resolved := flatResolveByName(file, resolver, rootReceiver)
	if resolved == nil || resolved.Kind == typeinfer.TypeUnknown {
		return false, false
	}
	if resolvedTypeMatches(resolved, sequenceExcludedTypes) {
		return true, false
	}
	if !resolvedTypeMatches(resolved, sequenceCandidateTypes) {
		return true, false
	}

	receiverType := resolved
	for _, call := range calls {
		name := flatCallExpressionName(file, call)
		if !sequenceOperationReturnsSequence(name) {
			return true, false
		}
		method := typeinfer.LookupStdlibMethod(simpleTypeReferenceName(receiverType.Name), name)
		if method == nil || method.ReturnType == nil || !resolvedTypeMatches(method.ReturnType, sequenceCandidateTypes) {
			return true, false
		}
		receiverType = method.ReturnType
	}
	return true, true
}

func collectionChainRootFlat(file *scanner.File, idx uint32) uint32 {
	current := idx
	for current != 0 && file.FlatType(current) == "call_expression" {
		navExpr, _ := flatCallExpressionParts(file, current)
		if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
			return current
		}
		receiver := file.FlatNamedChild(navExpr, 0)
		if receiver == 0 || file.FlatType(receiver) != "call_expression" {
			return receiver
		}
		current = receiver
	}
	return current
}

func matchesKnownTypeName(name string, candidates map[string]bool) bool {
	name = normalizeTypeReference(name)
	if name == "" {
		return false
	}
	if candidates[name] {
		return true
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 && candidates[name[idx+1:]] {
		return true
	}
	return false
}

func resolvedTypeMatches(resolved *typeinfer.ResolvedType, candidates map[string]bool) bool {
	if resolved == nil || resolved.Kind == typeinfer.TypeUnknown {
		return false
	}
	if matchesKnownTypeName(resolved.Name, candidates) || matchesKnownTypeName(resolved.FQN, candidates) {
		return true
	}
	for _, st := range resolved.Supertypes {
		if matchesKnownTypeName(st, candidates) {
			return true
		}
	}
	return false
}

func flatResolveByName(file *scanner.File, resolver typeinfer.TypeResolver, idx uint32) *typeinfer.ResolvedType {
	if resolver == nil || file == nil || idx == 0 {
		return nil
	}
	var name string
	switch file.FlatType(idx) {
	case "simple_identifier", "type_identifier":
		name = file.FlatNodeText(idx)
	case "navigation_expression":
		name = flatNavigationExpressionLastIdentifier(file, idx)
	case "user_type", "nullable_type":
		name = simpleTypeReferenceName(file.FlatNodeText(idx))
	case "call_expression":
		name = flatCallExpressionName(file, idx)
	default:
		name = strings.TrimSpace(file.FlatNodeText(idx))
	}
	if name == "" {
		return nil
	}
	return resolver.ResolveByNameFlat(name, idx, file)
}

func hasCollectionTypeAnnotation(functionText, name string) bool {
	for _, typeName := range collectionAnnotationTypeNames {
		if strings.Contains(functionText, name+": "+typeName) ||
			strings.Contains(functionText, name+": kotlin.collections."+typeName) {
			return true
		}
	}
	return false
}

func enclosingFunctionDeclarationFlat(file *scanner.File, idx uint32) uint32 {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "function_declaration" {
			return parent
		}
	}
	return 0
}
