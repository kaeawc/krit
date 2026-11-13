package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.hasAnnotationNamed
import dev.jasonpearson.krit.fir.support.identifierAnchor
import dev.jasonpearson.krit.fir.support.identifierText
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirEvaluatorResult
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.impl.FirDefaultPropertyGetter
import org.jetbrains.kotlin.fir.declarations.utils.isActual
import org.jetbrains.kotlin.fir.declarations.utils.isExpect
import org.jetbrains.kotlin.fir.declarations.utils.isExternal
import org.jetbrains.kotlin.fir.declarations.utils.isLateInit
import org.jetbrains.kotlin.fir.declarations.utils.isOpen
import org.jetbrains.kotlin.fir.declarations.utils.isOverride
import org.jetbrains.kotlin.fir.declarations.utils.hasExplicitBackingField
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirExpressionEvaluator
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.PrivateConstantEvaluatorAPI
import org.jetbrains.kotlin.fir.expressions.canBeUsedForConstVal
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid

// Flags a top-level, object or companion `val` that the compiler would accept as `const val`.
//
// Go matches the initializer's written shape (a literal, a sign, `+ - * / %` over those, and same-file
// references it can resolve) and works around tree-sitter recovery that turns constructor headers and
// stranded locals into properties. This checker asks the compiler the question `const val` itself asks:
// the declared type must be allowed for `const` (a primitive, an unsigned type or String, not nullable) and
// the compiler's constant evaluator must fold the initializer. That accepts folds Go never reads
// (`"a".length`, `1 shl 3`, `MAX + 1` over a `const`) and rejects what only looks constant, such as a
// reference to a `val` that is not `const` yet.
//
// A reference to a field of the app's generated `R` class is not a constant: AGP leaves those fields
// non-final by default, so the suggested `const` would not build. `android.R` stays constant.
//
// Not reported, like Go: `var`, an existing `const`, `override`/`open`, `lateinit`, `expect`/`actual`/
// `external`, a delegate, a custom getter, and `@JvmField`. Also not reported: `@JvmStatic`, which the
// compiler rejects on a `const`, so the suggested edit would not build. Class members outside an object,
// locals and members of anonymous objects are out of scope, as in Go.
internal object MayBeConstant : FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "MayBeConstant"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(MayBeConstant)
    }

    private val excludedAnnotations = setOf("JvmField", "JvmStatic")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        if (declaration.isLocal || declaration.isVar || !isCandidateDeclaration(declaration)) return
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind || source.elementType != KtNodeTypes.PROPERTY) return
        if (!isTopLevelOrObjectMember() || !canBeConst(declaration)) return
        val anchor = identifierAnchor(source) ?: return
        report(anchor, "${identifierText(source)} can be a `const val`.")
    }

    private fun isCandidateDeclaration(property: FirProperty): Boolean {
        val status = property.status
        if (status.isConst || status.isOverride || property.isOpen || status.isLateInit) return false
        if (status.isExpect || status.isActual || status.isExternal) return false
        if (property.delegate != null || property.hasExplicitBackingField) return false
        if (property.getter?.let { it !is FirDefaultPropertyGetter } == true) return false
        // K2 moves a field-targeted annotation such as @JvmField to the backing field.
        return !property.hasAnnotationNamed(excludedAnnotations) &&
            property.backingField?.hasAnnotationNamed(excludedAnnotations) != true
    }

    // A top-level property has no enclosing class; an object or companion member has a named object as owner.
    context(context: CheckerContext)
    private fun isTopLevelOrObjectMember(): Boolean {
        val owner = context.containingDeclarations.lastOrNull() ?: return false
        return when (owner) {
            is FirFileSymbol -> true
            is FirRegularClassSymbol -> owner.classKind == ClassKind.OBJECT
            else -> false
        }
    }

    @OptIn(PrivateConstantEvaluatorAPI::class)
    context(context: CheckerContext)
    private fun canBeConst(property: FirProperty): Boolean {
        val initializer = property.initializer ?: return false
        val type = property.returnTypeRef.coneType.fullyExpandedType()
        if (type.isMarkedNullable || !type.canBeUsedForConstVal() || readsGeneratedResourceId(initializer)) return false
        // The compiler's own `const val` check folds the initializer this way (FirConstPropertyChecker).
        return FirExpressionEvaluator.evaluateExpression(initializer, context.session) is FirEvaluatorResult.Evaluated
    }

    // AGP makes the fields of the app's own R class non-final by default, so `const val x = R.id.y` does not build.
    // The platform's android.R keeps final fields, so it stays a constant.
    private fun readsGeneratedResourceId(initializer: FirExpression): Boolean {
        var found = false
        initializer.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirQualifiedAccessExpression && isGeneratedResourceId(element)) found = true
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun isGeneratedResourceId(access: FirQualifiedAccessExpression): Boolean {
        val owner = access.calleeReference.toResolvedCallableSymbol()?.callableId?.classId?.outermostClassId ?: return false
        return owner.shortClassName.asString() == "R" && owner.packageFqName.asString() != "android"
    }
}
