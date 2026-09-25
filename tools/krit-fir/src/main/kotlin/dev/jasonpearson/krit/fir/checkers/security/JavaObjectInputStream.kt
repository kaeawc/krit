package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go JavaObjectInputStream rule: a constructor call that creates a
 * `java.io.ObjectInputStream` in production source, reported on the call.
 *
 * Mirrored from Go:
 * - only a call to an ObjectInputStream constructor counts. A subclass
 *   constructor (`FilteringInputStream(input)`), a supertype delegation
 *   (`class X(i: InputStream) : ObjectInputStream(i)`, `object :
 *   ObjectInputStream(i) {}`), and a callable reference (`::ObjectInputStream`)
 *   are not reported;
 * - test source files are skipped;
 * - a call whose nearest enclosing class (a class, interface, enum class, or
 *   annotation class, local ones included; objects, companion objects,
 *   anonymous objects, and enum-entry bodies are looked through, as Go's
 *   `class_declaration` walk does) is a filtering subclass is skipped. Go
 *   treats that class as one whose text mentions both `ObjectInputStream` and
 *   `resolveClass`; FIR requires it to extend java.io.ObjectInputStream and
 *   declare `resolveClass`.
 *
 * Deliberate differences from Go, pinned by goldens and listed in the PR:
 * - Precision: the constructed class must resolve to java.io.ObjectInputStream.
 *   Go accepts any call spelled `ObjectInputStream(...)` once the file imports
 *   or mentions the FQN (a `java.io.*` star import counts), so it also reports
 *   a local, nested, or same-package class (declared in this or another file),
 *   or a function, named ObjectInputStream.
 * - Recall: an import alias, a type alias, or a backticked name still creates
 *   an ObjectInputStream; Go matches the spelled name and misses them. Go's
 *   safe scope is a text match on the nearest class, and the call itself
 *   supplies the ObjectInputStream mention, so Go skips a raw call in any
 *   class whose text mentions `resolveClass`: one holding a nested filtering
 *   class or object, an object literal filter, or a companion filter; an
 *   interface or a plain class declaring a `resolveClass` helper; a comment or
 *   a string literal; an enum class whose other entry is a filter. None of
 *   those is a filtering subclass, so FIR reports the raw stream. A filtering
 *   subclass written as an object is not exempt either: exempting it would
 *   lose Go's finding in a top-level object filter.
 */
internal object JavaObjectInputStream : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "JavaObjectInputStream"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(JavaObjectInputStream)
    }

    private const val MESSAGE =
        "ObjectInputStream enables Java deserialization gadget attacks. Prefer JSON, protobuf, or kotlinx.serialization for untrusted data."

    private val objectInputStream = ClassId(FqName("java.io"), Name.identifier("ObjectInputStream"))
    private val resolveClass = Name.identifier("resolveClass")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (expression.calleeReference.toResolvedCallableSymbol() !is FirConstructorSymbol) return
        val constructed = expression.resolvedType.fullyExpandedType().lowerBoundIfFlexible()
        if (constructed.classId != objectInputStream) return
        if (isInTestFile()) return
        if (inFilteringSubclass()) return

        report(expression.source, MESSAGE)
    }

    // Go walks up to the nearest tree-sitter `class_declaration`, which is a
    // class, interface, enum class, or annotation class; `object`, companion
    // objects, object literals, and enum entries are other node types.
    // The nearest class is always a source class, so its direct declarations
    // are the members written in its body.
    @OptIn(DirectDeclarationsAccess::class)
    context(context: CheckerContext)
    private fun inFilteringSubclass(): Boolean {
        val nearest = context.containingDeclarations.asReversed()
            .firstOrNull { it is FirRegularClassSymbol && it.classKind != ClassKind.OBJECT && it.classKind != ClassKind.ENUM_ENTRY }
            as? FirRegularClassSymbol ?: return false
        val extendsObjectInputStream = lookupSuperTypes(
            nearest,
            lookupInterfaces = false,
            deep = true,
            useSiteSession = context.session,
        ).any { it.classId == objectInputStream }
        if (!extendsObjectInputStream) return false
        return nearest.declarationSymbols.any { it is FirNamedFunctionSymbol && it.name == resolveClass }
    }
}
