package dev.jasonpearson.krit.fir.checkers.naming

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.identifierAnchor
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isSomeFunctionType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.type
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Port of detekt's NonBooleanPropertyPrefixedWithIs and its Go twin: a property
 * whose name is `is` followed by an uppercase letter (`isName`, not `issues`)
 * and whose resolved type is not Boolean, reported on the property's name.
 *
 * Covered: every `val`/`var` declaration (top-level, member, local, anonymous
 * object, local class, enum entry), primary-constructor `val`/`var`
 * parameters, override, extension, delegated, and getter-backed properties,
 * and `when (val ...)` subjects. Skipped: loop variables, destructuring
 * entries, lambda and catch parameters, which are not properties.
 *
 * Boolean-ness follows detekt: the fully expanded type is `kotlin.Boolean`,
 * `java.lang.Boolean`, or an `AtomicBoolean` (java.util.concurrent, plus the
 * Kotlin and atomicfu ones), or a function type whose return type is one of
 * those (`() -> Boolean`, `KFunction0<Boolean>`). A type that is not a class
 * type (a type parameter, `T & Any`) or does not resolve is not reported,
 * because its type is unknown. An inferred `Nothing` is reported.
 *
 * Divergences from Go, which reads only the written type (see the golden data):
 * FIR also reports properties without a declared type, primary-constructor
 * properties, `when` subjects, backticked names, and a `Boolean` that names a
 * non-stdlib class.
 */
internal object NonBooleanPropertyPrefixedWithIs : FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "NonBooleanPropertyPrefixedWithIs"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(NonBooleanPropertyPrefixedWithIs)
    }

    private val booleanClassIds = setOf(
        StandardClassIds.Boolean,
        classIdOf("java.lang", "Boolean"),
        classIdOf("java.util.concurrent.atomic", "AtomicBoolean"),
        classIdOf("kotlin.concurrent.atomics", "AtomicBoolean"),
        classIdOf("kotlinx.atomicfu", "AtomicBoolean"),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        val name = declaration.name
        if (name.isSpecial) return
        val text = name.asString()
        if (!hasIsPrefix(text)) return
        val source = declaration.source ?: return
        val fromConstructor = source.kind == KtFakeSourceElementKind.PropertyFromParameter
        if (!fromConstructor && !isPropertyStatement(source)) return

        val type = declaration.returnTypeRef.coneType.fullyExpandedType().lowerBoundIfFlexible()
        if (type is ConeErrorType || type !is ConeClassLikeType) return
        if (isBooleanClass(type)) return
        val session = context.session
        if (type.isSomeFunctionType(session) && isBooleanClass(functionReturnType(type, session))) return

        report(identifierAnchor(source) ?: source, "Non-Boolean property '$text' should not be prefixed with 'is'")
    }

    // detekt's test: one UTF-16 unit after "is", uppercase per Character.isUpperCase.
    private fun hasIsPrefix(name: String): Boolean =
        name.length > 2 && name.startsWith("is") && Character.isUpperCase(name[2])

    // A function type's last type argument is its return type.
    private fun functionReturnType(type: ConeClassLikeType, session: FirSession): ConeKotlinType? =
        type.typeArguments.lastOrNull()?.type?.fullyExpandedType(session)?.lowerBoundIfFlexible()

    private fun isBooleanClass(type: ConeKotlinType?): Boolean =
        type is ConeClassLikeType && type !is ConeErrorType && type.lookupTag.classId in booleanClassIds

    // A real `val`/`var` declaration, including a `when (val x = ...)` subject.
    // Loop variables, destructuring entries, and catch parameters have other
    // source element types.
    private fun isPropertyStatement(source: KtSourceElement): Boolean =
        source.kind !is KtFakeSourceElementKind && source.elementType == KtNodeTypes.PROPERTY
}
