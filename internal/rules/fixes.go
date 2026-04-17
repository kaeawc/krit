package rules

// This file adds IsFixable() to rules that support auto-fix,
// and modifies their Check() methods to populate the Fix field.
// The actual Check() methods are in their original files (style.go, etc.)
// Here we only mark which rules are fixable.

// Fixable markers — rules implement FixableRule by having IsFixable().

func (r *WildcardImportRule) IsFixable() bool          { return false } // needs to resolve individual imports
func (r *ForbiddenCommentRule) IsFixable() bool         { return false } // human decision needed
func (r *MaxLineLengthRule) IsFixable() bool             { return false } // line breaking is formatter territory
func (r *NewLineAtEndOfFileRule) IsFixable() bool        { return true }
func (r *ForbiddenVoidRule) IsFixable() bool             { return true }
func (r *EqualsNullCallRule) IsFixable() bool            { return true }
func (r *RedundantHigherOrderMapUsageRule) IsFixable() bool { return true }
func (r *UnnecessaryFilterRule) IsFixable() bool         { return true }
func (r *UseCheckNotNullRule) IsFixable() bool           { return true }
func (r *UseRequireNotNullRule) IsFixable() bool         { return true }
func (r *UnnecessaryInheritanceRule) IsFixable() bool    { return true }

// New fixable rules
func (r *TrailingWhitespaceRule) IsFixable() bool                   { return true }
func (r *NoTabsRule) IsFixable() bool                               { return true }
func (r *RedundantVisibilityModifierRule) IsFixable() bool          { return true }
// RedundantConstructorKeyword: the Check() attempts a byte-mode fix
// but only when a `class_parameters` child is present in the
// constructor, which the current tree-sitter grammar doesn't produce
// reliably. Advertised as not-fixable until the parser pipeline
// matches.
func (r *RedundantConstructorKeywordRule) IsFixable() bool          { return false }
func (r *OptionalUnitRule) IsFixable() bool                         { return true }
func (r *ExplicitItLambdaParameterRule) IsFixable() bool            { return true }
func (r *RangeUntilInsteadOfRangeToRule) IsFixable() bool           { return true }
func (r *UseEmptyCounterpartRule) IsFixable() bool                  { return true }
func (r *UselessCallOnNotNullRule) IsFixable() bool                 { return true }
func (r *UnnecessaryApplyRule) IsFixable() bool                     { return true }
func (r *UseIsNullOrEmptyRule) IsFixable() bool                     { return true }
func (r *UseOrEmptyRule) IsFixable() bool                           { return true }
func (r *UseAnyOrNoneInsteadOfFindRule) IsFixable() bool            { return true }
func (r *UseCheckOrErrorRule) IsFixable() bool                      { return true }
func (r *UseRequireRule) IsFixable() bool                           { return true }
func (r *ExplicitCollectionElementAccessMethodRule) IsFixable() bool { return true }
func (r *UnnecessaryReversedRule) IsFixable() bool                  { return true }
func (r *UseSumOfInsteadOfFlatMapSizeRule) IsFixable() bool         { return true }
func (r *ArrayPrimitiveRule) IsFixable() bool                       { return true }
func (r *UnusedImportRule) IsFixable() bool                         { return true }
func (r *RedundantExplicitTypeRule) IsFixable() bool                { return true }

// Comments rules
func (r *DocumentationOverPrivateFunctionRule) IsFixable() bool { return true }
func (r *DocumentationOverPrivatePropertyRule) IsFixable() bool { return true }
func (r *DeprecatedBlockTagRule) IsFixable() bool               { return false }

// Empty-blocks rules
func (r *EmptyDefaultConstructorRule) IsFixable() bool      { return true }
func (r *EmptyInitBlockRule) IsFixable() bool                { return true }
func (r *EmptySecondaryConstructorRule) IsFixable() bool     { return true }
func (r *EmptyCatchBlockRule) IsFixable() bool               { return true }
func (r *EmptyClassBlockRule) IsFixable() bool               { return true }
func (r *EmptyDoWhileBlockRule) IsFixable() bool             { return true }
func (r *EmptyElseBlockRule) IsFixable() bool                { return true }
func (r *EmptyFinallyBlockRule) IsFixable() bool             { return true }
func (r *EmptyForBlockRule) IsFixable() bool                 { return true }
func (r *EmptyFunctionBlockRule) IsFixable() bool            { return true }
func (r *EmptyIfBlockRule) IsFixable() bool                  { return true }
func (r *EmptyTryBlockRule) IsFixable() bool                 { return true }
func (r *EmptyWhenBlockRule) IsFixable() bool                { return true }
func (r *EmptyWhileBlockRule) IsFixable() bool               { return true }

// Coroutines rules
func (r *SleepInsteadOfDelayRule) IsFixable() bool                   { return true }
func (r *SuspendFunWithFlowReturnTypeRule) IsFixable() bool          { return true }
func (r *SuspendFunWithCoroutineScopeReceiverRule) IsFixable() bool  { return true }
func (r *RedundantSuspendModifierRule) IsFixable() bool              { return true }

// Exceptions rules
func (r *PrintStackTraceRule) IsFixable() bool { return true }

// Performance rules
func (r *UnnecessaryPartOfBinaryExpressionRule) IsFixable() bool          { return true }
func (r *UnnecessaryTemporaryInstantiationRule) IsFixable() bool          { return true }

// Style2 — unused rules
//
// Unused-symbol deletions need multi-line span computation (leading
// KDoc, surrounding blank lines, follow-up comma cleanup in parameter
// lists) that the current Check() paths don't produce. They're
// advertised as not-fixable until the deletion pipeline lands.
// ModifierOrder remains fixable because it's an in-place rewrite of
// the modifier list with no span work.
func (r *UnusedVariableRule) IsFixable() bool         { return false }
func (r *UnusedParameterRule) IsFixable() bool        { return false }
func (r *UnusedPrivateClassRule) IsFixable() bool     { return false }
func (r *UnusedPrivateFunctionRule) IsFixable() bool  { return false }
func (r *UnusedPrivatePropertyRule) IsFixable() bool  { return false }
func (r *UnusedPrivateMemberRule) IsFixable() bool    { return false }
func (r *ModifierOrderRule) IsFixable() bool          { return true }

// Potential-bugs rules
func (r *UnsafeCastRule) IsFixable() bool                            { return true }
func (r *DoubleMutabilityForCollectionRule) IsFixable() bool         { return true }
func (r *CharArrayToStringCallRule) IsFixable() bool                 { return true }
func (r *AvoidReferentialEqualityRule) IsFixable() bool              { return true }
func (r *CastToNullableTypeRule) IsFixable() bool                    { return true }
func (r *UnnecessaryNotNullOperatorRule) IsFixable() bool            { return true }
func (r *UnnecessarySafeCallRule) IsFixable() bool                   { return true }
func (r *UnnecessaryNotNullCheckRule) IsFixable() bool               { return true }
func (r *MapGetWithNotNullAssertionRule) IsFixable() bool            { return true }
func (r *ImplicitUnitReturnTypeRule) IsFixable() bool                { return true }
func (r *ExplicitGarbageCollectionCallRule) IsFixable() bool         { return true }
func (r *UselessPostfixExpressionRule) IsFixable() bool              { return true }
func (r *NullableToStringCallRule) IsFixable() bool                  { return false }

// Style2 fixable rules
// AlsoCouldBeApply: `.also { }` → `.apply { }` swap is a simple
// text rewrite but the current Check() never populates Fix.
// Not-fixable until the helper lands.
func (r *AlsoCouldBeApplyRule) IsFixable() bool                     { return false }
func (r *DoubleNegativeExpressionRule) IsFixable() bool             { return true }
func (r *DoubleNegativeLambdaRule) IsFixable() bool                 { return false } // too complex
func (r *ExpressionBodySyntaxRule) IsFixable() bool                 { return true }
func (r *NullableBooleanCheckRule) IsFixable() bool                 { return true }
func (r *SpacingAfterPackageAndImportsRule) IsFixable() bool        { return true }
func (r *UnnecessaryAnyRule) IsFixable() bool                       { return true }
func (r *UnnecessaryBackticksRule) IsFixable() bool                 { return true }
func (r *UnnecessaryBracesAroundTrailingLambdaRule) IsFixable() bool { return true }
func (r *UnnecessaryLetRule) IsFixable() bool                       { return true }
func (r *UnnecessaryParenthesesRule) IsFixable() bool               { return true }
func (r *OptionalAbstractKeywordRule) IsFixable() bool              { return true }
func (r *MayBeConstantRule) IsFixable() bool                        { return true }
func (r *UnderscoresInNumericLiteralsRule) IsFixable() bool         { return true }
func (r *VarCouldBeValRule) IsFixable() bool                        { return true }

// Style2 fixable rules (batch 3)
// CollapsibleIfStatements: the Check() attempts a text-mode fix but
// relies on a parsing path that doesn't fire on the current
// tree-sitter output. Advertised as not-fixable until the AST shape
// is stable.
func (r *CollapsibleIfStatementsRule) IsFixable() bool              { return false }
func (r *DataClassShouldBeImmutableRule) IsFixable() bool           { return true }
func (r *ProtectedMemberInFinalClassRule) IsFixable() bool          { return true }
func (r *AbstractClassCanBeConcreteClassRule) IsFixable() bool      { return true }
func (r *ForbiddenImportRule) IsFixable() bool                      { return true }
func (r *ForbiddenAnnotationRule) IsFixable() bool                  { return true }
// UnnecessaryFullyQualifiedName: removing the FQN prefix requires
// ensuring an import already exists or adding one — the rule
// currently just reports. Not fixable until the import-management
// helper lands.
func (r *UnnecessaryFullyQualifiedNameRule) IsFixable() bool        { return false }
func (r *UnnecessaryInnerClassRule) IsFixable() bool                { return true }
func (r *TrimMultilineRawStringRule) IsFixable() bool               { return true }
func (r *UseArrayLiteralsInAnnotationsRule) IsFixable() bool        { return true }
func (r *EqualsOnSignatureLineRule) IsFixable() bool                { return true }

// Performance fixable rules (batch 3)
func (r *UnnecessaryTypeCastingRule) IsFixable() bool               { return true }
func (r *UnnecessaryInitOnArrayRule) IsFixable() bool               { return true }
func (r *CouldBeSequenceRule) IsFixable() bool                      { return false } // too complex: needs terminal operation awareness

// Potential-bugs fixable rules (batch 4)
// MissingPackageDeclaration: Check() derives a package fix from the
// file path via derivePackageFix, which only works when the path
// contains a recognizable source root (src/main/kotlin/...). Files
// outside those roots — including the fixture files themselves —
// get no Fix. Advertised as not-fixable because the fix isn't
// stable across inputs.
func (r *MissingPackageDeclarationRule) IsFixable() bool            { return false }

// Style2 fixable rules (batch 4)
func (r *MandatoryBracesLoopsRule) IsFixable() bool                 { return true }
// UseIfInsteadOfWhen: when → if rewrite requires rebuilding the
// condition + body structure; not fixable until the rewrite helper
// lands.
func (r *UseIfInsteadOfWhenRule) IsFixable() bool                   { return false }
// SerialVersionUIDInSerializableClass: fix would be to inject a new
// `private const val serialVersionUID = 1L` in the class body, but
// the current Check() never populates Fix. Advertised as not-fixable
// until the inject helper lands.
func (r *SerialVersionUIDInSerializableClassRule) IsFixable() bool  { return false }

// Style2 fixable rules (batch 5)
func (r *BracesOnIfStatementsRule) IsFixable() bool                 { return true }
func (r *BracesOnWhenStatementsRule) IsFixable() bool               { return true }
func (r *ClassOrderingRule) IsFixable() bool                        { return false } // too complex: reordering members with comments/annotations
func (r *AbstractClassCanBeInterfaceRule) IsFixable() bool          { return true }
// UseDataClass: converting a regular class to a data class requires
// rewriting the primary constructor and dropping equals/hashCode/
// toString members; not fixable until the class-rewrite helper lands.
func (r *UseDataClassRule) IsFixable() bool                         { return false }
func (r *UseIfEmptyOrIfBlankRule) IsFixable() bool                  { return true }
// UseLet: wrapping a block in `.let { }` is a structure rewrite that
// needs to pick an identifier and adjust the body — not fixable
// until the rewrite helper lands.
func (r *UseLetRule) IsFixable() bool                               { return false }
// FunctionOnlyReturningConstant: replacing a block body with an
// expression body needs the constant-only analysis the rule already
// does, but the current Check() never populates Fix. Advertised as
// not-fixable until the body-rewrite helper lands.
func (r *FunctionOnlyReturningConstantRule) IsFixable() bool        { return false }
// UtilityClassWithPublicConstructor: autofix landed in item 15
// Phase 2c (f3701ab) with two shapes — explicit visibility modifier
// swap and zero-width " private constructor()" insertion.
func (r *UtilityClassWithPublicConstructorRule) IsFixable() bool    { return true }
// ExplicitItLambdaMultipleParameters: naming the replacement
// parameter requires author intent; not fixable.
func (r *ExplicitItLambdaMultipleParametersRule) IsFixable() bool   { return false }

// Naming fixable rules
// BooleanPropertyNaming: the Check() attempts a name-rewrite fix but
// only when a `simple_identifier` is a direct child of the property
// declaration, which the current tree-sitter Kotlin grammar nests
// under `variable_declaration`. Advertised as not-fixable until the
// walker handles the nesting.
func (r *BooleanPropertyNamingRule) IsFixable() bool    { return false }

// Style2 fixable rules (batch 6)
func (r *ForbiddenMethodCallRule) IsFixable() bool      { return true }
func (r *ForbiddenSuppressRule) IsFixable() bool        { return true }
func (r *ForbiddenOptInRule) IsFixable() bool           { return true }

// Comments fixable rules (batch 2)
func (r *AbsentOrWrongFileLicenseRule) IsFixable() bool { return true }

// Exceptions rules (new fixable)
func (r *SwallowedExceptionRule) IsFixable() bool               { return true }
func (r *ReturnFromFinallyRule) IsFixable() bool                 { return true }
func (r *ThrowingExceptionFromFinallyRule) IsFixable() bool      { return true }

// Coroutines rules (new fixable)
func (r *GlobalCoroutineUsageRule) IsFixable() bool              { return true }
func (r *SuspendFunSwallowedCancellationRule) IsFixable() bool   { return true }

// Potential-bugs rules (new fixable)
func (r *DontDowncastCollectionTypesRule) IsFixable() bool       { return true }
func (r *CastNullableToNonNullableTypeRule) IsFixable() bool     { return true }
func (r *InvalidRangeRule) IsFixable() bool                      { return true }
func (r *WrongEqualsTypeParameterRule) IsFixable() bool          { return true }
func (r *UnreachableCodeRule) IsFixable() bool                   { return true }
func (r *MissingSuperCallRule) IsFixable() bool                  { return true }

// Performance rules (new fixable)
func (r *ForEachOnRangeRule) IsFixable() bool                    { return true }

// Style2 not fixable — too many edge cases
func (r *StringShouldBeRawStringRule) IsFixable() bool              { return false } // too complex: escape handling edge cases
func (r *MultilineLambdaItParameterRule) IsFixable() bool           { return false } // too complex: it-reference disambiguation
func (r *ObjectLiteralToLambdaRule) IsFixable() bool                { return false } // too complex: multi-method objects
func (r *SafeCastRule) IsFixable() bool                             { return false } // too complex: control flow rewriting
