package dev.jasonpearson.krit.fir

import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers

// K2 2.3.21 checker-set surface: 36 expression and 24 declaration properties.
// New rules contribute checker sets; only a K2 API change requires updating this file.
internal fun mergeFirRules(
    rules: List<FirRule>,
    baseExpressions: List<ExpressionCheckers> = emptyList(),
    baseDeclarations: List<DeclarationCheckers> = emptyList(),
): Pair<ExpressionCheckers, DeclarationCheckers> {
    val expressions = baseExpressions + rules.mapNotNull { it.expressionCheckers }
    val declarations = baseDeclarations + rules.mapNotNull { it.declarationCheckers }
    val expression = object : ExpressionCheckers() {
        override val basicExpressionCheckers = mergeSets(expressions) { it.basicExpressionCheckers }
        override val qualifiedAccessExpressionCheckers = mergeSets(expressions) { it.qualifiedAccessExpressionCheckers }
        override val callCheckers = mergeSets(expressions) { it.callCheckers }
        override val functionCallCheckers = mergeSets(expressions) { it.functionCallCheckers }
        override val propertyAccessExpressionCheckers = mergeSets(expressions) { it.propertyAccessExpressionCheckers }
        override val superReceiverExpressionCheckers = mergeSets(expressions) { it.superReceiverExpressionCheckers }
        override val integerLiteralOperatorCallCheckers = mergeSets(expressions) { it.integerLiteralOperatorCallCheckers }
        override val variableAssignmentCheckers = mergeSets(expressions) { it.variableAssignmentCheckers }
        override val tryExpressionCheckers = mergeSets(expressions) { it.tryExpressionCheckers }
        override val whenExpressionCheckers = mergeSets(expressions) { it.whenExpressionCheckers }
        override val loopExpressionCheckers = mergeSets(expressions) { it.loopExpressionCheckers }
        override val loopJumpCheckers = mergeSets(expressions) { it.loopJumpCheckers }
        override val booleanOperatorExpressionCheckers = mergeSets(expressions) { it.booleanOperatorExpressionCheckers }
        override val returnExpressionCheckers = mergeSets(expressions) { it.returnExpressionCheckers }
        override val blockCheckers = mergeSets(expressions) { it.blockCheckers }
        override val replDeclarationReferenceCheckers = mergeSets(expressions) { it.replDeclarationReferenceCheckers }
        override val annotationCheckers = mergeSets(expressions) { it.annotationCheckers }
        override val annotationCallCheckers = mergeSets(expressions) { it.annotationCallCheckers }
        override val checkNotNullCallCheckers = mergeSets(expressions) { it.checkNotNullCallCheckers }
        override val elvisExpressionCheckers = mergeSets(expressions) { it.elvisExpressionCheckers }
        override val getClassCallCheckers = mergeSets(expressions) { it.getClassCallCheckers }
        override val safeCallExpressionCheckers = mergeSets(expressions) { it.safeCallExpressionCheckers }
        override val smartCastExpressionCheckers = mergeSets(expressions) { it.smartCastExpressionCheckers }
        override val equalityOperatorCallCheckers = mergeSets(expressions) { it.equalityOperatorCallCheckers }
        override val stringConcatenationCallCheckers = mergeSets(expressions) { it.stringConcatenationCallCheckers }
        override val typeOperatorCallCheckers = mergeSets(expressions) { it.typeOperatorCallCheckers }
        override val resolvedQualifierCheckers = mergeSets(expressions) { it.resolvedQualifierCheckers }
        override val literalExpressionCheckers = mergeSets(expressions) { it.literalExpressionCheckers }
        override val callableReferenceAccessCheckers = mergeSets(expressions) { it.callableReferenceAccessCheckers }
        override val thisReceiverExpressionCheckers = mergeSets(expressions) { it.thisReceiverExpressionCheckers }
        override val whileLoopCheckers = mergeSets(expressions) { it.whileLoopCheckers }
        override val throwExpressionCheckers = mergeSets(expressions) { it.throwExpressionCheckers }
        override val doWhileLoopCheckers = mergeSets(expressions) { it.doWhileLoopCheckers }
        override val collectionLiteralCheckers = mergeSets(expressions) { it.collectionLiteralCheckers }
        override val classReferenceExpressionCheckers = mergeSets(expressions) { it.classReferenceExpressionCheckers }
        override val inaccessibleReceiverCheckers = mergeSets(expressions) { it.inaccessibleReceiverCheckers }
    }
    val declaration = object : DeclarationCheckers() {
        override val basicDeclarationCheckers = mergeSets(declarations) { it.basicDeclarationCheckers }
        override val callableDeclarationCheckers = mergeSets(declarations) { it.callableDeclarationCheckers }
        override val functionCheckers = mergeSets(declarations) { it.functionCheckers }
        override val simpleFunctionCheckers = mergeSets(declarations) { it.simpleFunctionCheckers }
        override val propertyCheckers = mergeSets(declarations) { it.propertyCheckers }
        override val classLikeCheckers = mergeSets(declarations) { it.classLikeCheckers }
        override val classCheckers = mergeSets(declarations) { it.classCheckers }
        override val regularClassCheckers = mergeSets(declarations) { it.regularClassCheckers }
        override val constructorCheckers = mergeSets(declarations) { it.constructorCheckers }
        override val fileCheckers = mergeSets(declarations) { it.fileCheckers }
        override val scriptCheckers = mergeSets(declarations) { it.scriptCheckers }
        override val replSnippetCheckers = mergeSets(declarations) { it.replSnippetCheckers }
        override val typeParameterCheckers = mergeSets(declarations) { it.typeParameterCheckers }
        override val typeAliasCheckers = mergeSets(declarations) { it.typeAliasCheckers }
        override val anonymousFunctionCheckers = mergeSets(declarations) { it.anonymousFunctionCheckers }
        override val propertyAccessorCheckers = mergeSets(declarations) { it.propertyAccessorCheckers }
        override val backingFieldCheckers = mergeSets(declarations) { it.backingFieldCheckers }
        override val valueParameterCheckers = mergeSets(declarations) { it.valueParameterCheckers }
        override val enumEntryCheckers = mergeSets(declarations) { it.enumEntryCheckers }
        override val anonymousObjectCheckers = mergeSets(declarations) { it.anonymousObjectCheckers }
        override val anonymousInitializerCheckers = mergeSets(declarations) { it.anonymousInitializerCheckers }
        override val receiverParameterCheckers = mergeSets(declarations) { it.receiverParameterCheckers }
        override val controlFlowAnalyserCheckers = mergeSets(declarations) { it.controlFlowAnalyserCheckers }
        override val variableAssignmentCfaBasedCheckers = mergeSets(declarations) { it.variableAssignmentCfaBasedCheckers }
    }
    return expression to declaration
}

private fun <S, T> mergeSets(sets: List<S>, get: (S) -> Set<T>): Set<T> =
    sets.flatMap(get).toSet()
