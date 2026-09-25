package dev.jasonpearson.krit.fir

import org.jetbrains.kotlin.fir.analysis.cfa.AbstractFirPropertyInitializationChecker
import org.jetbrains.kotlin.fir.analysis.checkers.cfa.FirControlFlowChecker
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.analysis.checkers.type.FirTypeChecker
import org.jetbrains.kotlin.fir.analysis.checkers.type.TypeCheckers
import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.expressions.FirStatement
import org.jetbrains.kotlin.fir.types.FirTypeRef

internal data class MergedFirCheckers(
    val expression: ExpressionCheckers,
    val declaration: DeclarationCheckers,
    val type: TypeCheckers,
)

// K2 2.3.21 checker-set surface: 36 expression, 24 declaration and 4 type
// properties. New rules contribute checker sets; only a K2 API change requires
// updating this file, and FirCheckerSurfaceTest fails when one is missed.
//
// With a [recorder] (every check compile), each rule's checkers are wrapped so
// an exception thrown by one is recorded against that rule and file and
// swallowed (see FirRuleIsolation.kt) instead of crashing the compile, which
// would void the FIR verdict for every file and rule. Base checkers are the
// krit-fir infrastructure itself and stay unwrapped.
internal fun mergeFirRules(
    rules: List<FirRule>,
    baseExpressions: List<ExpressionCheckers> = emptyList(),
    baseDeclarations: List<DeclarationCheckers> = emptyList(),
    baseTypes: List<TypeCheckers> = emptyList(),
    recorder: FirRuleErrorRecorder? = null,
): MergedFirCheckers {
    val expressions = baseExpressions.map { Contribution(null, it) } +
        rules.mapNotNull { rule -> rule.expressionCheckers?.let { Contribution(rule.ruleId, it) } }
    val declarations = baseDeclarations.map { Contribution(null, it) } +
        rules.mapNotNull { rule -> rule.declarationCheckers?.let { Contribution(rule.ruleId, it) } }
    val types = baseTypes.map { Contribution(null, it) } +
        rules.mapNotNull { rule -> rule.typeCheckers?.let { Contribution(rule.ruleId, it) } }

    fun <E : FirStatement> ex(get: (ExpressionCheckers) -> Set<FirExpressionChecker<E>>) =
        mergeSets(expressions, recorder, get) { id, c, r -> IsolatedExpressionChecker(id, c, r) }
    fun <D : FirDeclaration> de(get: (DeclarationCheckers) -> Set<FirDeclarationChecker<D>>) =
        mergeSets(declarations, recorder, get) { id, c, r -> IsolatedDeclarationChecker(id, c, r) }
    fun cfa(get: (DeclarationCheckers) -> Set<FirControlFlowChecker>) =
        mergeSets(declarations, recorder, get) { id, c, r -> IsolatedControlFlowChecker(id, c, r) }
    fun init(get: (DeclarationCheckers) -> Set<AbstractFirPropertyInitializationChecker>) =
        mergeSets(declarations, recorder, get) { id, c, r -> IsolatedPropertyInitializationChecker(id, c, r) }
    fun <T : FirTypeRef> ty(get: (TypeCheckers) -> Set<FirTypeChecker<T>>) =
        mergeSets(types, recorder, get) { id, c, r -> IsolatedTypeChecker(id, c, r) }

    val expression = object : ExpressionCheckers() {
        override val basicExpressionCheckers = ex { it.basicExpressionCheckers }
        override val qualifiedAccessExpressionCheckers = ex { it.qualifiedAccessExpressionCheckers }
        override val callCheckers = ex { it.callCheckers }
        override val functionCallCheckers = ex { it.functionCallCheckers }
        override val propertyAccessExpressionCheckers = ex { it.propertyAccessExpressionCheckers }
        override val superReceiverExpressionCheckers = ex { it.superReceiverExpressionCheckers }
        override val integerLiteralOperatorCallCheckers = ex { it.integerLiteralOperatorCallCheckers }
        override val variableAssignmentCheckers = ex { it.variableAssignmentCheckers }
        override val tryExpressionCheckers = ex { it.tryExpressionCheckers }
        override val whenExpressionCheckers = ex { it.whenExpressionCheckers }
        override val loopExpressionCheckers = ex { it.loopExpressionCheckers }
        override val loopJumpCheckers = ex { it.loopJumpCheckers }
        override val booleanOperatorExpressionCheckers = ex { it.booleanOperatorExpressionCheckers }
        override val returnExpressionCheckers = ex { it.returnExpressionCheckers }
        override val blockCheckers = ex { it.blockCheckers }
        override val replDeclarationReferenceCheckers = ex { it.replDeclarationReferenceCheckers }
        override val annotationCheckers = ex { it.annotationCheckers }
        override val annotationCallCheckers = ex { it.annotationCallCheckers }
        override val checkNotNullCallCheckers = ex { it.checkNotNullCallCheckers }
        override val elvisExpressionCheckers = ex { it.elvisExpressionCheckers }
        override val getClassCallCheckers = ex { it.getClassCallCheckers }
        override val safeCallExpressionCheckers = ex { it.safeCallExpressionCheckers }
        override val smartCastExpressionCheckers = ex { it.smartCastExpressionCheckers }
        override val equalityOperatorCallCheckers = ex { it.equalityOperatorCallCheckers }
        override val stringConcatenationCallCheckers = ex { it.stringConcatenationCallCheckers }
        override val typeOperatorCallCheckers = ex { it.typeOperatorCallCheckers }
        override val resolvedQualifierCheckers = ex { it.resolvedQualifierCheckers }
        override val literalExpressionCheckers = ex { it.literalExpressionCheckers }
        override val callableReferenceAccessCheckers = ex { it.callableReferenceAccessCheckers }
        override val thisReceiverExpressionCheckers = ex { it.thisReceiverExpressionCheckers }
        override val whileLoopCheckers = ex { it.whileLoopCheckers }
        override val throwExpressionCheckers = ex { it.throwExpressionCheckers }
        override val doWhileLoopCheckers = ex { it.doWhileLoopCheckers }
        override val collectionLiteralCheckers = ex { it.collectionLiteralCheckers }
        override val classReferenceExpressionCheckers = ex { it.classReferenceExpressionCheckers }
        override val inaccessibleReceiverCheckers = ex { it.inaccessibleReceiverCheckers }
    }
    val declaration = object : DeclarationCheckers() {
        override val basicDeclarationCheckers = de { it.basicDeclarationCheckers }
        override val callableDeclarationCheckers = de { it.callableDeclarationCheckers }
        override val functionCheckers = de { it.functionCheckers }
        override val simpleFunctionCheckers = de { it.simpleFunctionCheckers }
        override val propertyCheckers = de { it.propertyCheckers }
        override val classLikeCheckers = de { it.classLikeCheckers }
        override val classCheckers = de { it.classCheckers }
        override val regularClassCheckers = de { it.regularClassCheckers }
        override val constructorCheckers = de { it.constructorCheckers }
        override val fileCheckers = de { it.fileCheckers }
        override val scriptCheckers = de { it.scriptCheckers }
        override val replSnippetCheckers = de { it.replSnippetCheckers }
        override val typeParameterCheckers = de { it.typeParameterCheckers }
        override val typeAliasCheckers = de { it.typeAliasCheckers }
        override val anonymousFunctionCheckers = de { it.anonymousFunctionCheckers }
        override val propertyAccessorCheckers = de { it.propertyAccessorCheckers }
        override val backingFieldCheckers = de { it.backingFieldCheckers }
        override val valueParameterCheckers = de { it.valueParameterCheckers }
        override val enumEntryCheckers = de { it.enumEntryCheckers }
        override val anonymousObjectCheckers = de { it.anonymousObjectCheckers }
        override val anonymousInitializerCheckers = de { it.anonymousInitializerCheckers }
        override val receiverParameterCheckers = de { it.receiverParameterCheckers }
        override val controlFlowAnalyserCheckers = cfa { it.controlFlowAnalyserCheckers }
        override val variableAssignmentCfaBasedCheckers = init { it.variableAssignmentCfaBasedCheckers }
    }
    val type = object : TypeCheckers() {
        override val typeRefCheckers = ty { it.typeRefCheckers }
        override val resolvedTypeRefCheckers = ty { it.resolvedTypeRefCheckers }
        override val functionTypeRefCheckers = ty { it.functionTypeRefCheckers }
        override val intersectionTypeRefCheckers = ty { it.intersectionTypeRefCheckers }
    }
    return MergedFirCheckers(expression, declaration, type)
}

/** A checker-set contribution; [ruleId] is null for krit-fir's own base checkers. */
private class Contribution<S>(val ruleId: String?, val checkers: S)

private fun <S, T> mergeSets(
    sources: List<Contribution<S>>,
    recorder: FirRuleErrorRecorder?,
    get: (S) -> Set<T>,
    isolate: (String, T, FirRuleErrorRecorder) -> T,
): Set<T> =
    sources.flatMap { source ->
        val checkers = get(source.checkers)
        val ruleId = source.ruleId
        if (ruleId == null || recorder == null) checkers else checkers.map { isolate(ruleId, it, recorder) }
    }.toSet()
