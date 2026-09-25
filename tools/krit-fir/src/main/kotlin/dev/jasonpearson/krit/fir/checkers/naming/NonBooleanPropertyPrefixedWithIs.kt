package dev.jasonpearson.krit.fir.checkers.naming

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Port of the Go NonBooleanPropertyPrefixedWithIs rule: a property whose name
 * starts with `is` and whose type is not Boolean, reported on the property's
 * first line (its modifier list, or `val`/`var` when it has none), like Go.
 *
 * Covered like Go: every `val`/`var` declaration statement, wherever it
 * sits (top-level, class, object, companion, interface, anonymous object,
 * local class, enum entry, and function-local variables), override and
 * private properties, extension, delegated, and getter-backed properties, and
 * names that merely start with the letters `is` (`issues`), since Go matches
 * the prefix literally. Skipped like Go: loop variables, destructuring
 * entries, catch parameters, and `when (val ...)` subjects, which are not
 * property declarations in Go's tree.
 *
 * Deliberate precision differences from Go, which decides Boolean-ness from
 * the declared type's text (`Boolean`/`Boolean?`) or a literal `true`/`false`
 * initializer and otherwise requires the text `": "` somewhere in the
 * declaration (see the golden data):
 * - a property whose resolved type is Boolean is not reported even when Go
 *   cannot see it (`kotlin.Boolean`, a type alias of Boolean, `java.lang.Boolean`,
 *   or an inferred Boolean whose initializer happens to contain `": "`);
 * - a non-Boolean property is reported even when Go cannot see it: an inferred
 *   type with no `": "` in the declaration, a declared type written without a
 *   space after the colon, a `Boolean` that resolves to a same-named
 *   non-stdlib class, and primary-constructor `val`/`var` properties (Go only
 *   visits property declarations, not class parameters).
 * An inferred `Nothing` type (`val isReady = TODO()`) is not reported, like
 * Go: it says nothing about the intended type.
 */
internal object NonBooleanPropertyPrefixedWithIs : FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "NonBooleanPropertyPrefixedWithIs"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(NonBooleanPropertyPrefixedWithIs)
    }

    private val javaLangBoolean = ClassId(FqName("java.lang"), Name.identifier("Boolean"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        val name = declaration.name
        if (name.isSpecial) return
        val text = name.asString()
        if (!text.startsWith("is")) return
        val source = declaration.source ?: return
        val fromConstructor = source.kind == KtFakeSourceElementKind.PropertyFromParameter
        if (!fromConstructor && !isPropertyStatement(source)) return

        val typeRef = declaration.returnTypeRef
        val type = typeRef.coneType.fullyExpandedType().lowerBoundIfFlexible()
        if (type is ConeErrorType) return
        if (isBoolean(type)) return
        val implicitType = typeRef.source.let { it == null || it.kind is KtFakeSourceElementKind } && !fromConstructor
        if (implicitType && type.classId == StandardClassIds.Nothing) return

        report(source, "Non-Boolean property '$text' should not be prefixed with 'is'")
    }

    private fun isBoolean(type: ConeKotlinType): Boolean {
        if (type !is ConeClassLikeType) return false
        val classId = type.lookupTag.classId
        return classId == StandardClassIds.Boolean || classId == javaLangBoolean
    }

    // A real `val`/`var` declaration statement: Go's property_declaration.
    // Loop variables, destructuring entries, and catch parameters have other
    // source element types; a `when (val x = ...)` subject is a PROPERTY in
    // Kotlin's tree but not a property_declaration in Go's.
    private fun isPropertyStatement(source: KtSourceElement): Boolean {
        if (source.kind is KtFakeSourceElementKind) return false
        if (source.elementType != KtNodeTypes.PROPERTY) return false
        val parent = source.treeStructure.getParent(source.lighterASTNode) ?: return true
        return parent.tokenType != KtNodeTypes.WHEN
    }
}
