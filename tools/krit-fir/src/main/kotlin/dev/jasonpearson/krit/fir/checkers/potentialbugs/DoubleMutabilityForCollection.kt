package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toRegularClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.upperBoundIfFlexible
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Port of the Go DoubleMutabilityForCollection rule: a `var` whose type is a
 * mutable collection, reported on the property's first line (its modifier
 * list, or `var` when it has none), like Go.
 *
 * Go reports a `var` property declaration when its declared type's simple
 * name is one of the `mutableTypes` option's simple names (and, for the
 * built-in collection names, the name is not shadowed in the file), or,
 * failing that, when its initializer is a call named like a mutable
 * collection factory (`mutableListOf`, `hashMapOf`, `ArrayList`, ...). The
 * checker decides both from resolution:
 * - the property's type (declared or inferred, type aliases expanded) is a
 *   configured mutable type: a built-in name (`MutableList`, `ArrayList`,
 *   ...) means the `kotlin.collections` / `java.util` class of that name,
 *   another qualified name means that class, and an unqualified one any class
 *   with that simple name;
 * - or the initializer is a call named like one of Go's factories and the
 *   property's type is itself a mutable collection (a `MutableIterable` or
 *   `MutableMap`), which keeps Go's factory findings on mutable types the
 *   option does not list (`var c: MutableCollection<T> = mutableListOf()`, a
 *   `ConcurrentHashMap` imported as `HashMap`).
 *
 * Covered like Go: top-level, member, companion, object, interface, and local
 * `var`s, nullable types, `lateinit`, override, getter-backed, extension, and
 * delegated properties, and test files are skipped. Not reported, like Go:
 * `val`s and destructuring declarations.
 *
 * Deliberate differences from Go (see the golden data):
 * - a mutable collection type Go cannot see from the declaration's text is
 *   reported: an inferred type from a call Go does not list
 *   (`mutableListOf<T>().apply { }`, `list.toMutableList()`), a type alias or
 *   import alias of a mutable collection, a built-in name in a file with an
 *   unrelated star import or a same-named declaration elsewhere in the file,
 *   and a primary-constructor `var` parameter;
 * - Go's factory-name findings whose property is not a mutable collection are
 *   dropped: a read-only declared type (`var xs: List<T> = mutableListOf()`)
 *   and a same-named function or class that builds no mutable collection
 *   (`fun mutableListOf(): Int`).
 * - a qualified non-built-in `mutableTypes` entry matches that class only; Go
 *   matches any class with its simple name.
 */
internal object DoubleMutabilityForCollection : FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "DoubleMutabilityForCollection"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(DoubleMutabilityForCollection)
    }

    private const val MESSAGE = "Variable with mutable collection type creates double mutability. " +
        "Use val with a mutable collection or var with an immutable collection."

    // Go's defaultDoubleMutableTypes: the struct default, used when the option
    // is present but empty.
    private val goStructDefault = listOf(
        "MutableList", "MutableSet", "MutableMap", "MutableCollection",
        "ArrayList", "HashMap", "HashSet", "LinkedHashMap", "LinkedHashSet",
    )

    // config/default-krit.yml's list, the operative default when no option is
    // sent (a bare compile or the golden tests).
    private val yamlDefault = listOf(
        "kotlin.collections.MutableList",
        "kotlin.collections.MutableMap",
        "kotlin.collections.MutableSet",
        "java.util.ArrayList",
        "java.util.LinkedHashSet",
        "java.util.HashSet",
        "java.util.LinkedHashMap",
        "java.util.HashMap",
    )

    // Go's knownMutableCollectionFQN: the classes a built-in simple name stands
    // for. The kotlin.collections ArrayList/HashMap/... entries are type
    // aliases of the java.util classes.
    private val knownMutableCollections: Map<String, Set<FqName>> = listOf(
        "kotlin.collections.MutableList",
        "kotlin.collections.MutableSet",
        "kotlin.collections.MutableMap",
        "kotlin.collections.MutableCollection",
        "kotlin.collections.ArrayList",
        "kotlin.collections.HashMap",
        "kotlin.collections.HashSet",
        "kotlin.collections.LinkedHashMap",
        "kotlin.collections.LinkedHashSet",
        "java.util.ArrayList",
        "java.util.HashMap",
        "java.util.HashSet",
        "java.util.LinkedHashMap",
        "java.util.LinkedHashSet",
    ).map { FqName(it) }.groupBy({ it.shortName().asString() }, { it }).mapValues { it.value.toSet() }

    // Go's mutableCollectionFactories.
    private val factoryNames = setOf(
        "mutableListOf", "mutableSetOf", "mutableMapOf", "arrayListOf",
        "hashMapOf", "hashSetOf", "linkedMapOf", "linkedSetOf",
        "ArrayList", "HashMap", "HashSet", "LinkedHashMap", "LinkedHashSet",
    )

    private val mutableRoots = setOf(StandardClassIds.MutableIterable, StandardClassIds.MutableMap)

    /** One `mutableTypes` entry, resolved to what it matches. */
    private sealed interface TypeMatcher {
        fun matches(fqName: FqName): Boolean
    }

    private class FqNames(val names: Set<FqName>) : TypeMatcher {
        override fun matches(fqName: FqName) = fqName in names
    }

    private class SimpleName(val name: String) : TypeMatcher {
        override fun matches(fqName: FqName) = fqName.shortName().asString() == name
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        if (!declaration.isVar) return
        val source = declaration.source ?: return
        val fromConstructor = source.kind == KtFakeSourceElementKind.PropertyFromParameter
        if (!fromConstructor) {
            // A real `var` declaration statement, Go's property_declaration;
            // destructuring entries have another element type.
            if (source.kind is KtFakeSourceElementKind) return
            if (source.elementType != KtNodeTypes.PROPERTY) return
        }
        val session = context.session
        val type = declaration.returnTypeRef.coneType
        val reported = isConfiguredMutableType(type, matchers(), session) ||
            (isFactoryNamedCall(declaration.initializer) && isMutableCollection(type, session))
        if (!reported) return
        // Go skips scanner.IsTestFile files; the check request carries that
        // classification (configured test paths included).
        if (isInTestFile()) return

        report(source, MESSAGE)
    }

    private fun matchers(): List<TypeMatcher> {
        val option = config()["mutableTypes"]
        val entries = when {
            option !is List<*> -> yamlDefault
            else -> option.filterIsInstance<String>().ifEmpty { goStructDefault }
        }
        return entries.mapNotNull { entry -> matcherFor(entry) }
    }

    // Go reduces every entry to its simple name (dropping `?` and type
    // arguments); a built-in collection name stands for the known
    // kotlin.collections / java.util classes whatever the entry's qualifier.
    private fun matcherFor(entry: String): TypeMatcher? {
        var text = entry.trim().removeSuffix("?")
        text = text.substringBefore('<').trim()
        if (text.isEmpty()) return null
        val simple = text.substringAfterLast('.').trim()
        if (simple.isEmpty()) return null
        knownMutableCollections[simple]?.let { return FqNames(it) }
        return if ('.' in text) FqNames(setOf(FqName(text))) else SimpleName(simple)
    }

    // Every bound of the type (a flexible Java type has two) names a
    // configured class, before or after type-alias expansion.
    private fun isConfiguredMutableType(type: ConeKotlinType, matchers: List<TypeMatcher>, session: FirSession): Boolean {
        val written = type.lowerBoundIfFlexible() as? ConeClassLikeType
        val writtenName = written?.lookupTag?.classId?.asSingleFqName()
        val bounds = expandedBounds(type, session) ?: return false
        return bounds.all { bound ->
            val name = bound.lookupTag.classId.asSingleFqName()
            matchers.any { it.matches(name) || (writtenName != null && it.matches(writtenName)) }
        }
    }

    private fun isMutableCollection(type: ConeKotlinType, session: FirSession): Boolean {
        val bounds = expandedBounds(type, session) ?: return false
        return bounds.all { bound ->
            val classId = bound.lookupTag.classId
            if (classId in mutableRoots) return@all true
            val symbol = bound.lookupTag.toRegularClassSymbol(session) ?: return@all false
            lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
                .any { it.lookupTag.classId in mutableRoots }
        }
    }

    private fun expandedBounds(type: ConeKotlinType, session: FirSession): List<ConeClassLikeType>? {
        val expanded = type.fullyExpandedType(session)
        val bounds = listOf(expanded.lowerBoundIfFlexible(), expanded.upperBoundIfFlexible()).distinct()
        return bounds.map { it as? ConeClassLikeType ?: return null }
    }

    // The initializer is a call named like a mutable collection factory,
    // Go's mutableCollectionFactories test on the callee's last name segment,
    // whatever the call resolves to (the caller also requires the property's
    // type to be a mutable collection).
    private fun isFactoryNamedCall(initializer: FirExpression?): Boolean {
        val expression = (initializer as? FirSafeCallExpression)?.selector ?: initializer
        val call = expression as? FirFunctionCall ?: return false
        return call.calleeReference.name.asString() in factoryNames
    }
}
