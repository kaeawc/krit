package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirRegularClassChecker
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.constructors
import org.jetbrains.kotlin.fir.declarations.hasAnnotation
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go Instantiatable rule: an Android component class (an
 * Activity, Service, BroadcastReceiver, ContentProvider, or Application) that
 * the framework cannot instantiate because the class is `private` or its
 * primary constructor is `private`. Like Go, only class-like declarations are
 * visited (classes at any nesting, including local classes; not `object`s),
 * a private class is reported whatever its constructors, and the finding sits
 * on the declaration's first line (its modifier list, else the `class`
 * keyword) with Go's message.
 *
 * The component proof is the class's superclass closure: one of
 * `android.app.Activity`, `android.app.Service`,
 * `android.content.BroadcastReceiver`, `android.content.ContentProvider`, or
 * `android.app.Application`. Every name in Go's list (`AppCompatActivity`,
 * `ComponentActivity`, `FragmentActivity`, `IntentService`, ...) extends one
 * of them.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`Instantiatable*.kt`):
 * - Precision: Go matches a direct supertype by its simple name, so it
 *   reports a class extending a project or third-party class named `Service`,
 *   `Application`, `Activity`, and so on. That class is not an Android
 *   component, so it is not reported here.
 * - Precision: Go reports a private primary constructor even when the class
 *   also declares a public (or internal, public in bytecode) secondary
 *   constructor the framework can call with no arguments. That class can be
 *   instantiated, so it is not reported here.
 * - Recall: Go reads only the direct supertype's written name, so it misses a
 *   component reached through a project base class, another framework
 *   subclass (`ListActivity`, a third-party Application subclass), a type
 *   alias, or an import alias. Those are reported here.
 * - Recall: Go reads only a primary constructor's modifiers, so it misses a
 *   class with no primary constructor whose secondary constructors are all
 *   private. The framework cannot instantiate it either, so it is reported
 *   here. A public constructor that takes arguments stays unreported, as in
 *   Go: a custom AppComponentFactory can call it.
 */
internal object Instantiatable : FirRegularClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "Instantiatable"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val regularClassCheckers = setOf(Instantiatable)
    }

    private const val MESSAGE =
        "This class is registered as an Android component but cannot be instantiated. " +
            "Remove the private constructor or add a public no-arg constructor."

    private val componentClassIds = setOf(
        ClassId(FqName("android.app"), Name.identifier("Activity")),
        ClassId(FqName("android.app"), Name.identifier("Service")),
        ClassId(FqName("android.app"), Name.identifier("Application")),
        ClassId(FqName("android.content"), Name.identifier("BroadcastReceiver")),
        ClassId(FqName("android.content"), Name.identifier("ContentProvider")),
    )

    private val jvmOverloadsClassId = ClassId(FqName("kotlin.jvm"), Name.identifier("JvmOverloads"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirRegularClass) {
        // Go visits class_declaration nodes only, never object declarations.
        if (declaration.classKind == ClassKind.OBJECT) return
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val session = context.session
        if (!isComponent(declaration, session)) return
        val constructors = declaration.constructors(session)
        val privateConstructor = constructors.any { it.isExplicitPrivatePrimary() } ||
            (constructors.none { it.isPrimary } && constructors.isNotEmpty() && constructors.all { it.isPrivate() })
        val notInstantiatable = declaration.status.visibility == Visibilities.Private ||
            (privateConstructor && constructors.none { it.isPublicNoArg(session) })
        if (!notInstantiatable) return
        report(firstLine(source), MESSAGE)
    }

    // Supertypes are read from their own lookup tags, so no class id is
    // resolved from a symbol that may be local.
    private fun isComponent(declaration: FirRegularClass, session: FirSession): Boolean {
        for (ref in declaration.superTypeRefs) {
            val type = ref.coneType.fullyExpandedType(session).lowerBoundIfFlexible() as? ConeClassLikeType ?: continue
            if (type.lookupTag.classId in componentClassIds) return true
            val symbol = type.lookupTag.toClassSymbol(session) ?: continue
            val inherits = lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = session)
                .any { it.lookupTag.classId in componentClassIds }
            if (inherits) return true
        }
        return false
    }

    // A primary constructor written in source with `private`, as Go reads it
    // (the implicit default constructor is never private here).
    private fun FirConstructorSymbol.isExplicitPrivatePrimary(): Boolean =
        isPrimary && source?.kind is KtRealSourceElementKind && isPrivate()

    private fun FirConstructorSymbol.isPrivate(): Boolean = resolvedStatus.visibility == Visibilities.Private

    // A secondary constructor the framework can call reflectively with no
    // arguments: public in bytecode (public or internal) and taking no
    // arguments, or taking only defaulted ones under @JvmOverloads.
    private fun FirConstructorSymbol.isPublicNoArg(session: FirSession): Boolean {
        if (isPrimary) return false
        val visibility = resolvedStatus.visibility
        if (visibility != Visibilities.Public && visibility != Visibilities.Internal) return false
        val params = valueParameterSymbols
        if (params.isEmpty()) return true
        return params.all { it.hasDefaultValue } && hasAnnotation(jvmOverloadsClassId, session)
    }

    // Go reports the class_declaration's first line: the first entry of its
    // modifier list (an annotation or a modifier keyword) when it has one,
    // else the `class` / `interface` keyword.
    private fun firstLine(source: KtSourceElement): KtSourceElement {
        val children = lightChildren(source, source.lighterASTNode)
        val modifiers = children.firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST }
        val anchor = modifiers?.let { list ->
            lightChildren(source, list).firstOrNull {
                it.tokenType !in KtTokens.WHITESPACES && it.tokenType !in KtTokens.COMMENTS
            }
        } ?: modifiers
            ?: children.firstOrNull { it.tokenType == KtTokens.CLASS_KEYWORD || it.tokenType == KtTokens.INTERFACE_KEYWORD }
            ?: return source
        return lightSourceOf(anchor, source)
    }
}
