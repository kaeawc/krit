package rules

import v2 "github.com/kaeawc/krit/internal/rules/v2"

// FixLevel indicates how safe an auto-fix is.
type FixLevel int

const (
	// FixCosmetic: whitespace, formatting, comments only. Cannot change behavior.
	FixCosmetic FixLevel = 1
	// FixIdiomatic: idiomatic transforms producing semantically equivalent code.
	FixIdiomatic FixLevel = 2
	// FixSemantic: correct in most cases but could change edge-case behavior
	// (reflection, serialization, binary compatibility, identity semantics).
	FixSemantic FixLevel = 3
)

func (l FixLevel) String() string {
	switch l {
	case FixCosmetic:
		return "cosmetic"
	case FixIdiomatic:
		return "idiomatic"
	case FixSemantic:
		return "semantic"
	default:
		return "unknown"
	}
}

// ParseFixLevel parses a fix level string.
func ParseFixLevel(s string) (FixLevel, bool) {
	switch s {
	case "cosmetic":
		return FixCosmetic, true
	case "idiomatic":
		return FixIdiomatic, true
	case "semantic":
		return FixSemantic, true
	default:
		return 0, false
	}
}

// Fixable rules declare their safety level structurally by exposing a
// `FixLevel() FixLevel` method. GetFixLevel type-asserts to an anonymous
// interface at the call site; rules without the method default to FixSemantic.

// v2FixLevelCarrier is the interface implemented by v2 compat wrappers that
// carry a fix level from the underlying v2.Rule. It returns the level as an
// int to avoid an import cycle between rules and rules/v2. A zero value means
// the wrapped rule declared no fix level.
type v2FixLevelCarrier interface {
	V1FixLevel() int
}

// GetFixLevel returns the fix level of a rule (defaults to FixSemantic).
func GetFixLevel(r Rule) FixLevel {
	if fl, ok := r.(interface{ FixLevel() FixLevel }); ok {
		return fl.FixLevel()
	}
	if v, ok := r.(v2FixLevelCarrier); ok {
		if lvl := v.V1FixLevel(); lvl != 0 {
			return FixLevel(lvl)
		}
	}
	return FixSemantic
}

// GetV2FixLevel returns the fix level for a v2 rule. It reads r.Fix when set
// (non-zero), otherwise falls back to the v1 OriginalV1 FixLevel() method.
// Returns (0, false) when the rule is not fixable.
func GetV2FixLevel(r *v2.Rule) (FixLevel, bool) {
	if r == nil {
		return 0, false
	}
	// Fast path: fix level already encoded in v2.Rule.Fix.
	if r.Fix != v2.FixNone {
		return FixLevel(r.Fix), true
	}
	// Fall back to v1 OriginalV1 FixLevel method.
	if r.OriginalV1 != nil {
		if fl, ok := r.OriginalV1.(interface{ FixLevel() FixLevel }); ok {
			lvl := fl.FixLevel()
			if lvl != 0 {
				return lvl, true
			}
		}
	}
	return 0, false
}

// --- cosmetic: whitespace, formatting, comments only ---

func (r *TrailingWhitespaceRule) FixLevel() FixLevel           { return FixCosmetic }
func (r *NoTabsRule) FixLevel() FixLevel                       { return FixCosmetic }
func (r *NewLineAtEndOfFileRule) FixLevel() FixLevel           { return FixCosmetic }
func (r *SpacingAfterPackageAndImportsRule) FixLevel() FixLevel { return FixCosmetic }
func (r *UnderscoresInNumericLiteralsRule) FixLevel() FixLevel { return FixCosmetic }
func (r *ModifierOrderRule) FixLevel() FixLevel                { return FixCosmetic }
func (r *EqualsOnSignatureLineRule) FixLevel() FixLevel        { return FixCosmetic }
func (r *RedundantVisibilityModifierRule) FixLevel() FixLevel  { return FixCosmetic }
func (r *RedundantConstructorKeywordRule) FixLevel() FixLevel  { return FixCosmetic }
func (r *RedundantExplicitTypeRule) FixLevel() FixLevel        { return FixCosmetic }
func (r *OptionalUnitRule) FixLevel() FixLevel                 { return FixCosmetic }
func (r *OptionalAbstractKeywordRule) FixLevel() FixLevel      { return FixCosmetic }
func (r *UnnecessaryBackticksRule) FixLevel() FixLevel         { return FixCosmetic }
func (r *UnnecessaryParenthesesRule) FixLevel() FixLevel       { return FixCosmetic }
func (r *DocumentationOverPrivateFunctionRule) FixLevel() FixLevel  { return FixCosmetic }
func (r *DocumentationOverPrivatePropertyRule) FixLevel() FixLevel  { return FixCosmetic }
func (r *MandatoryBracesLoopsRule) FixLevel() FixLevel         { return FixCosmetic }
func (r *TrimMultilineRawStringRule) FixLevel() FixLevel       { return FixCosmetic }
func (r *ExplicitItLambdaParameterRule) FixLevel() FixLevel    { return FixCosmetic }
func (r *BracesOnIfStatementsRule) FixLevel() FixLevel                  { return FixCosmetic }
func (r *BracesOnWhenStatementsRule) FixLevel() FixLevel                { return FixCosmetic }
func (r *ExplicitItLambdaMultipleParametersRule) FixLevel() FixLevel    { return FixCosmetic }
func (r *UnnecessaryBracesAroundTrailingLambdaRule) FixLevel() FixLevel { return FixCosmetic }
func (r *MissingPackageDeclarationRule) FixLevel() FixLevel    { return FixCosmetic }

// --- safe: semantically equivalent transforms ---

func (r *UseCheckNotNullRule) FixLevel() FixLevel              { return FixIdiomatic }
func (r *UseRequireNotNullRule) FixLevel() FixLevel            { return FixIdiomatic }
func (r *UseCheckOrErrorRule) FixLevel() FixLevel              { return FixIdiomatic }
func (r *UseRequireRule) FixLevel() FixLevel                   { return FixIdiomatic }
func (r *UseIsNullOrEmptyRule) FixLevel() FixLevel             { return FixIdiomatic }
func (r *UseOrEmptyRule) FixLevel() FixLevel                   { return FixIdiomatic }
func (r *UseAnyOrNoneInsteadOfFindRule) FixLevel() FixLevel    { return FixIdiomatic }
func (r *UseIfEmptyOrIfBlankRule) FixLevel() FixLevel          { return FixIdiomatic }
func (r *UseLetRule) FixLevel() FixLevel                       { return FixIdiomatic }
func (r *UseEmptyCounterpartRule) FixLevel() FixLevel          { return FixIdiomatic }
func (r *UseArrayLiteralsInAnnotationsRule) FixLevel() FixLevel { return FixIdiomatic }
func (r *UseSumOfInsteadOfFlatMapSizeRule) FixLevel() FixLevel { return FixIdiomatic }
func (r *EqualsNullCallRule) FixLevel() FixLevel               { return FixIdiomatic }
func (r *UnnecessaryFilterRule) FixLevel() FixLevel            { return FixIdiomatic }
func (r *UnnecessaryInheritanceRule) FixLevel() FixLevel       { return FixIdiomatic }
func (r *RedundantHigherOrderMapUsageRule) FixLevel() FixLevel { return FixIdiomatic }
func (r *UnnecessaryApplyRule) FixLevel() FixLevel             { return FixIdiomatic }
func (r *UnnecessaryLetRule) FixLevel() FixLevel               { return FixIdiomatic }
func (r *UnnecessaryAnyRule) FixLevel() FixLevel               { return FixIdiomatic }
func (r *UnnecessaryReversedRule) FixLevel() FixLevel          { return FixIdiomatic }
func (r *UnnecessaryPartOfBinaryExpressionRule) FixLevel() FixLevel { return FixIdiomatic }
func (r *UnnecessaryTemporaryInstantiationRule) FixLevel() FixLevel { return FixIdiomatic }
func (r *UnnecessaryTypeCastingRule) FixLevel() FixLevel       { return FixIdiomatic }
func (r *UnnecessaryInitOnArrayRule) FixLevel() FixLevel       { return FixIdiomatic }
func (r *UnnecessaryFullyQualifiedNameRule) FixLevel() FixLevel { return FixIdiomatic }
func (r *UnnecessaryInnerClassRule) FixLevel() FixLevel        { return FixIdiomatic }
func (r *UselessCallOnNotNullRule) FixLevel() FixLevel         { return FixIdiomatic }
func (r *UselessPostfixExpressionRule) FixLevel() FixLevel     { return FixIdiomatic }
func (r *ForbiddenVoidRule) FixLevel() FixLevel                { return FixIdiomatic }
func (r *RangeUntilInsteadOfRangeToRule) FixLevel() FixLevel   { return FixIdiomatic }
func (r *ExplicitCollectionElementAccessMethodRule) FixLevel() FixLevel { return FixIdiomatic }
func (r *ExpressionBodySyntaxRule) FixLevel() FixLevel         { return FixIdiomatic }
func (r *DoubleNegativeExpressionRule) FixLevel() FixLevel     { return FixIdiomatic }
func (r *NullableBooleanCheckRule) FixLevel() FixLevel         { return FixIdiomatic }
func (r *MayBeConstantRule) FixLevel() FixLevel                { return FixIdiomatic }
func (r *VarCouldBeValRule) FixLevel() FixLevel                { return FixIdiomatic }
func (r *ArrayPrimitiveRule) FixLevel() FixLevel               { return FixIdiomatic }
func (r *CharArrayToStringCallRule) FixLevel() FixLevel        { return FixIdiomatic }
func (r *SleepInsteadOfDelayRule) FixLevel() FixLevel          { return FixIdiomatic }
func (r *SuspendFunWithFlowReturnTypeRule) FixLevel() FixLevel { return FixIdiomatic }
func (r *SuspendFunWithCoroutineScopeReceiverRule) FixLevel() FixLevel { return FixIdiomatic }
func (r *RedundantSuspendModifierRule) FixLevel() FixLevel     { return FixIdiomatic }
func (r *CollapsibleIfStatementsRule) FixLevel() FixLevel      { return FixIdiomatic }
func (r *UseIfInsteadOfWhenRule) FixLevel() FixLevel           { return FixIdiomatic }
func (r *ImplicitUnitReturnTypeRule) FixLevel() FixLevel       { return FixIdiomatic }
func (r *ExplicitGarbageCollectionCallRule) FixLevel() FixLevel { return FixIdiomatic }
func (r *UnusedImportRule) FixLevel() FixLevel                 { return FixIdiomatic }
func (r *ForbiddenImportRule) FixLevel() FixLevel              { return FixIdiomatic }
func (r *ForbiddenAnnotationRule) FixLevel() FixLevel          { return FixIdiomatic }

// --- cosmetic: license header ---

func (r *AbsentOrWrongFileLicenseRule) FixLevel() FixLevel     { return FixCosmetic } // inserts license comment

// --- semantic: naming / annotation removal ---

func (r *BooleanPropertyNamingRule) FixLevel() FixLevel        { return FixSemantic } // renames property (could break callers)
func (r *ForbiddenMethodCallRule) FixLevel() FixLevel          { return FixSemantic } // deletes code
func (r *ForbiddenSuppressRule) FixLevel() FixLevel            { return FixSemantic } // removes suppression (may surface warnings)
func (r *ForbiddenOptInRule) FixLevel() FixLevel               { return FixSemantic } // removes opt-in (may surface errors)

// --- cautious: could change edge-case behavior ---

func (r *AvoidReferentialEqualityRule) FixLevel() FixLevel           { return FixSemantic } // changes identity to equality
func (r *CastToNullableTypeRule) FixLevel() FixLevel                 { return FixSemantic } // changes cast semantics
func (r *UnsafeCastRule) FixLevel() FixLevel                         { return FixSemantic } // adds null to return type
func (r *DoubleMutabilityForCollectionRule) FixLevel() FixLevel      { return FixSemantic } // var→val may break reassignment
func (r *DataClassShouldBeImmutableRule) FixLevel() FixLevel         { return FixSemantic } // var→val in data class breaks callers
func (r *ProtectedMemberInFinalClassRule) FixLevel() FixLevel        { return FixSemantic } // protected→private breaks subclass access
func (r *AbstractClassCanBeConcreteClassRule) FixLevel() FixLevel    { return FixSemantic } // removes abstract, changes instantiation
func (r *UnnecessaryNotNullOperatorRule) FixLevel() FixLevel         { return FixSemantic } // removes !!, could NPE
func (r *UnnecessarySafeCallRule) FixLevel() FixLevel                { return FixSemantic } // ?.→., could NPE if analysis wrong
func (r *UnnecessaryNotNullCheckRule) FixLevel() FixLevel            { return FixSemantic } // removes null check
func (r *MapGetWithNotNullAssertionRule) FixLevel() FixLevel { return FixSemantic } // changes exception type on missing key
func (r *PrintStackTraceRule) FixLevel() FixLevel                    { return FixSemantic } // removes logging
func (r *EmptyCatchBlockRule) FixLevel() FixLevel                    { return FixSemantic } // adds TODO, changes empty behavior
func (r *EmptyFunctionBlockRule) FixLevel() FixLevel                 { return FixSemantic } // adds TODO()
func (r *EmptyClassBlockRule) FixLevel() FixLevel                    { return FixSemantic } // removes class body
func (r *EmptyDoWhileBlockRule) FixLevel() FixLevel                  { return FixSemantic } // removes loop
func (r *EmptyElseBlockRule) FixLevel() FixLevel                     { return FixSemantic } // removes else branch
func (r *EmptyFinallyBlockRule) FixLevel() FixLevel                  { return FixSemantic } // removes finally
func (r *EmptyForBlockRule) FixLevel() FixLevel                      { return FixSemantic } // removes loop
func (r *EmptyIfBlockRule) FixLevel() FixLevel                       { return FixSemantic } // removes if
func (r *EmptyTryBlockRule) FixLevel() FixLevel                      { return FixSemantic } // removes try
func (r *EmptyWhenBlockRule) FixLevel() FixLevel                     { return FixSemantic } // removes when
func (r *EmptyWhileBlockRule) FixLevel() FixLevel                    { return FixSemantic } // removes loop
func (r *EmptyDefaultConstructorRule) FixLevel() FixLevel            { return FixSemantic } // removes constructor
func (r *EmptyInitBlockRule) FixLevel() FixLevel                     { return FixSemantic } // removes init block
func (r *EmptySecondaryConstructorRule) FixLevel() FixLevel          { return FixSemantic } // removes constructor body
func (r *SerialVersionUIDInSerializableClassRule) FixLevel() FixLevel { return FixSemantic } // adds code
func (r *UnusedParameterRule) FixLevel() FixLevel                    { return FixSemantic } // renames param (could break named args)
func (r *UnusedVariableRule) FixLevel() FixLevel                     { return FixSemantic } // renames var
func (r *UnusedPrivateClassRule) FixLevel() FixLevel                 { return FixSemantic } // deletes code
func (r *UnusedPrivateFunctionRule) FixLevel() FixLevel              { return FixSemantic } // deletes code
func (r *UnusedPrivatePropertyRule) FixLevel() FixLevel              { return FixSemantic } // deletes code
func (r *UnusedPrivateMemberRule) FixLevel() FixLevel                { return FixSemantic } // deletes code
func (r *AlsoCouldBeApplyRule) FixLevel() FixLevel                   { return FixSemantic } // changes receiver scope
func (r *AbstractClassCanBeInterfaceRule) FixLevel() FixLevel        { return FixSemantic } // changes class to interface
func (r *UseDataClassRule) FixLevel() FixLevel                       { return FixSemantic } // adds data modifier
func (r *FunctionOnlyReturningConstantRule) FixLevel() FixLevel      { return FixSemantic } // changes API from fun to val
func (r *UtilityClassWithPublicConstructorRule) FixLevel() FixLevel  { return FixSemantic } // adds private constructor

// --- new fixable rules ---

func (r *SwallowedExceptionRule) FixLevel() FixLevel                  { return FixSemantic } // adds throw in catch
func (r *SuspendFunSwallowedCancellationRule) FixLevel() FixLevel     { return FixSemantic } // adds throw in catch
func (r *ReturnFromFinallyRule) FixLevel() FixLevel                   { return FixSemantic } // removes return from finally
func (r *ThrowingExceptionFromFinallyRule) FixLevel() FixLevel        { return FixSemantic } // removes throw from finally
func (r *GlobalCoroutineUsageRule) FixLevel() FixLevel                { return FixSemantic } // removes GlobalScope prefix
func (r *DontDowncastCollectionTypesRule) FixLevel() FixLevel         { return FixSemantic } // changes cast to copy
func (r *CastNullableToNonNullableTypeRule) FixLevel() FixLevel       { return FixSemantic } // changes unsafe cast to safe cast
func (r *WrongEqualsTypeParameterRule) FixLevel() FixLevel            { return FixSemantic } // changes equals parameter type
func (r *UnreachableCodeRule) FixLevel() FixLevel                     { return FixSemantic } // deletes unreachable code
func (r *MissingSuperCallRule) FixLevel() FixLevel                    { return FixSemantic } // adds super call
func (r *InvalidRangeRule) FixLevel() FixLevel                        { return FixIdiomatic } // replaces .. with downTo
func (r *ForEachOnRangeRule) FixLevel() FixLevel                      { return FixIdiomatic } // replaces forEach with for loop
