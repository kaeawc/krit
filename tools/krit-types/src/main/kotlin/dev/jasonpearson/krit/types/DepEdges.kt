package dev.jasonpearson.krit.types

import com.intellij.psi.PsiComment
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiWhiteSpace
import com.intellij.psi.impl.source.tree.LeafPsiElement
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.analysis.api.KaSession
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.types.*
import org.jetbrains.kotlin.idea.references.KtReference
import org.jetbrains.kotlin.idea.references.mainReference
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.psi.*

// Source-dependency edges for the Go-side oracle cache.
//
// An edge from the analyzed file F to a source file D says F's facts can
// depend on D. The cache fingerprints F's closure: its direct edges plus,
// transitively, each dependency's *propagating* edges. An edge is
// propagating when a change in D can also change what a file depending on F
// sees, which is the case for edges recorded in F's API:
//
//   - declaration signatures: types, supertypes, type-parameter bounds,
//     annotations and their arguments, typealias right-hand sides;
//   - parameter default values;
//   - bodies whose type is inferred (expression-bodied functions without a
//     return type, properties without a type), `const` initializers, `inline`
//     function and accessor bodies, and bodies declaring a contract.
//
// Other body edges (an explicitly typed function's body, an initializer
// block, a secondary constructor, arguments to a supertype constructor) and
// import edges affect only F's own facts. The outermost body decides: a local
// declaration or lambda inside an explicitly typed body never propagates,
// and anything inside an inferred body always does. Visibility is ignored;
// a private inferred declaration can feed a non-private one's inferred type.

/** Where a PSI subtree sits relative to its declaration's API. */
internal enum class EdgeScope(val propagating: Boolean, val fixed: Boolean) {
    /** Declaration signature (not inside any body): propagates. */
    SIGNATURE(propagating = true, fixed = false),
    /** Inside a body whose contents leak into dependents' view. */
    PROPAGATING_BODY(propagating = true, fixed = true),
    /** Inside a body only this file's facts depend on, or an import. */
    LOCAL_BODY(propagating = false, fixed = true),
}

internal object DepEdgeScopes {
    /**
     * The scope of [child] when it is a body slot of [parent], or null when
     * [child] is not a body of [parent] (so it keeps the parent's scope).
     */
    fun bodySlotScope(parent: PsiElement, child: PsiElement): EdgeScope? {
        val propagates = when (parent) {
            is KtNamedFunction -> if (child === parent.bodyExpression) functionBodyPropagates(parent) else return null
            is KtPropertyAccessor -> if (child === parent.bodyExpression) accessorBodyPropagates(parent) else return null
            is KtProperty -> if (child === parent.initializer || child === parent.delegate) propertyValuePropagates(parent) else return null
            is KtParameter -> if (child === parent.defaultValue) true else return null
            is KtAnonymousInitializer -> if (child === parent.body) false else return null
            is KtSecondaryConstructor -> if (child === parent.bodyExpression || child is KtConstructorDelegationCall) false else return null
            is KtSuperTypeCallEntry -> if (child === parent.valueArgumentList || child is KtLambdaArgument) false else return null
            is KtDelegatedSuperTypeEntry -> if (child === parent.delegateExpression) false else return null
            else -> return null
        }
        return if (propagates) EdgeScope.PROPAGATING_BODY else EdgeScope.LOCAL_BODY
    }

    fun functionBodyPropagates(function: KtNamedFunction): Boolean =
        (function.typeReference == null && !function.hasBlockBody()) ||
            function.hasModifier(KtTokens.INLINE_KEYWORD) ||
            declaresContract(function)

    fun accessorBodyPropagates(accessor: KtPropertyAccessor): Boolean {
        val property = accessor.property
        if (accessor.hasModifier(KtTokens.INLINE_KEYWORD) || property.hasModifier(KtTokens.INLINE_KEYWORD)) return true
        // A getter's body is the property's type when neither declares one.
        return accessor.isGetter && property.typeReference == null && accessor.typeReference == null
    }

    fun propertyValuePropagates(property: KtProperty): Boolean =
        property.typeReference == null || property.hasModifier(KtTokens.CONST_KEYWORD)

    /** `contract { ... }` (optionally qualified) as the body's first statement. */
    fun declaresContract(function: KtNamedFunction): Boolean {
        val first = function.bodyBlockExpression?.statements?.firstOrNull() ?: return false
        val call = when (first) {
            is KtCallExpression -> first
            is KtDotQualifiedExpression -> first.selectorExpression as? KtCallExpression
            else -> null
        } ?: return false
        return call.calleeExpression?.text == "contract" && call.lambdaArguments.isNotEmpty()
    }
}

/**
 * Walks [file] once, top-down, resolving every reference that can reach
 * another source file and recording it in [tracker] with the scope of the
 * position it came from. Scopes are passed down the walk, so classifying a
 * position costs nothing per node.
 */
@OptIn(KaExperimentalApi::class)
internal fun KaSession.recordResolvedDependencyEdges(file: KtFile, tracker: DepTracker, perf: KotlinPerf?) {
    val start = System.nanoTime()
    val owner = file.virtualFilePath

    fun recordPath(symbol: KaSymbol?, propagating: Boolean) {
        dependencySourcePathOf(symbol)?.let { tracker.recordDepPath(owner, it, propagating) }
    }

    // Every source class named in [type], its type arguments, and the
    // typealias it was written as. A callable's declaring file does not
    // always cover the classes its type names: a Java declaration has no
    // dependency fragment of its own, so a Kotlin class reached only through
    // a Java method's return type would otherwise be missing from the
    // closure.
    fun recordType(type: KaType?, propagating: Boolean, depth: Int = 0) {
        if (type == null || depth > 8) return
        when (type) {
            is KaClassType -> {
                recordPath(type.symbol, propagating)
                for (argument in type.typeArguments) {
                    recordType((argument as? KaTypeArgumentWithVariance)?.type, propagating, depth + 1)
                }
            }
            is KaFlexibleType -> {
                recordType(type.lowerBound, propagating, depth + 1)
                recordType(type.upperBound, propagating, depth + 1)
            }
            is KaDefinitelyNotNullType -> recordType(type.original, propagating, depth + 1)
            is KaIntersectionType -> type.conjuncts.forEach { recordType(it, propagating, depth + 1) }
            else -> {}
        }
        type.abbreviation?.let { recordPath(it.symbol, propagating) }
    }

    fun recordSignatureTypes(callable: KaCallableSymbol, propagating: Boolean) {
        try {
            recordType(callable.returnType, propagating)
            recordType(callable.receiverParameter?.returnType, propagating)
        } catch (_: Throwable) {}
    }

    fun recordSymbol(symbol: KaSymbol, propagating: Boolean) {
        // The resolved symbol's types may be substituted (a generic member
        // seen through a subclass); the declaration's are recorded below.
        if (symbol is KaCallableSymbol) recordSignatureTypes(symbol, propagating)
        var declaring: KaSymbol = symbol
        if (declaring is KaConstructorSymbol) {
            declaring = runCatching { declaring.originalConstructorIfTypeAliased }.getOrNull() ?: declaring
        }
        if (declaring is KaCallableSymbol) {
            val original = runCatching { declaring.fakeOverrideOriginal }.getOrNull()
            if (original != null && original !== declaring) {
                recordSignatureTypes(original, propagating)
                declaring = original
            }
        }
        recordPath(declaring, propagating)
        when (declaring) {
            // A default constructor may have no PSI of its own; its class does.
            is KaConstructorSymbol ->
                dependencySourcePathOf((declaring.returnType as? KaClassType)?.symbol)?.let { tracker.recordDepPath(owner, it, propagating) }
            // A typealias use depends on what it expands to as well.
            is KaTypeAliasSymbol ->
                dependencySourcePathOf((declaring.expandedType as? KaClassType)?.symbol)?.let { tracker.recordDepPath(owner, it, propagating) }
            else -> {}
        }
    }

    fun resolve(reference: KtReference?, propagating: Boolean) {
        if (reference == null) return
        val symbols = try {
            reference.resolveToSymbols()
        } catch (_: Throwable) {
            return
        }
        for (symbol in symbols) {
            try {
                recordSymbol(symbol, propagating)
            } catch (_: Throwable) {}
        }
    }

    fun visitReferences(element: PsiElement, propagating: Boolean) {
        when (element) {
            is KtLabelReferenceExpression -> {}
            is KtOperationReferenceExpression ->
                // Built-in operators have no symbol to reach; overloadable
                // ones may resolve to a source operator function.
                if (element.operationSignTokenType !in nonOverloadableOperators) resolve(element.mainReference, propagating)
            is KtSimpleNameExpression -> resolve(element.mainReference, propagating)
            // Operator conventions with no name at the use site: an
            // extension operator in another file is only reachable here.
            is KtArrayAccessExpression -> resolve(element.mainReference, propagating)
            is KtForExpression, is KtPropertyDelegate, is KtDestructuringDeclarationEntry ->
                for (ref in element.references) if (ref is KtReference) resolve(ref, propagating)
        }
    }

    fun walk(element: PsiElement, scope: EdgeScope) {
        visitReferences(element, scope.propagating)
        var child = element.firstChild
        while (child != null) {
            if (child !is PsiWhiteSpace && child !is PsiComment && child !is LeafPsiElement) {
                val childScope = if (scope.fixed) scope else DepEdgeScopes.bodySlotScope(element, child) ?: scope
                walk(child, childScope)
            }
            child = child.nextSibling
        }
    }

    var top = file.firstChild
    while (top != null) {
        when (top) {
            is KtPackageDirective, is PsiWhiteSpace, is PsiComment, is LeafPsiElement -> {}
            is KtImportList -> walk(top, EdgeScope.LOCAL_BODY)
            else -> walk(top, EdgeScope.SIGNATURE)
        }
        top = top.nextSibling
    }
    if (perf != null) {
        val elapsed = System.nanoTime() - start
        perf.count("kotlinDepEdges.file", elapsed)
        perf.addPhaseTotal("kotlinDepEdges", elapsed)
    }
}

private val nonOverloadableOperators = setOf(
    KtTokens.ANDAND, KtTokens.OROR, KtTokens.EQEQEQ, KtTokens.EXCLEQEQEQ, KtTokens.ELVIS,
    KtTokens.EQ, KtTokens.AS_KEYWORD, KtTokens.AS_SAFE, KtTokens.IS_KEYWORD, KtTokens.NOT_IS,
    KtTokens.EXCLEXCL,
)

/**
 * The source file declaring [symbol], for symbols compiled from this
 * module's Kotlin or Java sources. Compiler-generated members of source
 * classes (data class `copy`/`componentN`, enum `values`) count as their
 * class's file.
 */
internal fun dependencySourcePathOf(symbol: KaSymbol?): String? {
    if (symbol == null) return null
    when (symbol.origin) {
        KaSymbolOrigin.SOURCE, KaSymbolOrigin.SOURCE_MEMBER_GENERATED, KaSymbolOrigin.JAVA_SOURCE -> {}
        else -> return null
    }
    val psi: PsiElement = symbol.psi ?: return null
    return psi.containingFile?.virtualFile?.path
}
