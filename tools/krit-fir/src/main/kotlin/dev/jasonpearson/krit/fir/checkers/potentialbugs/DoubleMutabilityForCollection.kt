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
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.abbreviatedType
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
 * - every bound of the property's type (declared or inferred; a Java platform
 *   type has two) is a configured mutable type, by its name as written or
 *   after type-alias expansion: a built-in name (`MutableList`, `ArrayList`,
 *   ...) means the `kotlin.collections` / `java.util` class of that name, or
 *   any other class or type alias of that simple name that is a mutable
 *   collection (Go only sees a shadowing declaration in the same file);
 *   another qualified name means that class, and an unqualified one any class
 *   or type alias with that simple name. A `(Mutable)List<T>!` from a Java
 *   `java.util.List` return is not reported: its upper bound is read-only;
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
 *   a same-named mutable collection declared in the file, a parenthesized
 *   type or initializer, a Java `java.util.ArrayList` return
 *   (`Collections.list(e)`), and a primary-constructor `var` parameter;
 * - Go's factory-name findings whose property is not a mutable collection are
 *   dropped: a read-only declared type (`var xs: List<T> = mutableListOf()`)
 *   and a same-named function or class that builds no mutable collection
 *   (`fun mutableListOf(): Int`), and a built-in name that resolves to a
 *   non-collection declared in another file of the package;
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

    /**
     * One `mutableTypes` entry, resolved to what it matches. [matches] gets the
     * names of one bound of the property's type (as written and after
     * type-alias expansion) and whether that bound is a mutable collection.
     */
    private sealed interface TypeMatcher {
        fun matches(names: List<FqName>, isMutable: () -> Boolean): Boolean
    }

    // A built-in collection name: the kotlin.collections / java.util classes
    // of that name, or any other class or type alias of that simple name that
    // is a mutable collection. Go matches the written simple name and only
    // treats it as shadowed by a declaration in the same file, so a
    // same-package `class LinkedHashMap<K, V> : java.util.LinkedHashMap<K, V>()`
    // declared in another file is still a Go finding, and a true one.
    private class BuiltIn(val simpleName: String, val fqNames: Set<FqName>) : TypeMatcher {
        override fun matches(names: List<FqName>, isMutable: () -> Boolean) =
            names.any { it in fqNames } ||
                (names.any { it.shortName().asString() == simpleName } && isMutable())
    }

    private class FqNames(val fqNames: Set<FqName>) : TypeMatcher {
        override fun matches(names: List<FqName>, isMutable: () -> Boolean) = names.any { it in fqNames }
    }

    private class SimpleName(val name: String) : TypeMatcher {
        override fun matches(names: List<FqName>, isMutable: () -> Boolean) =
            names.any { it.shortName().asString() == name }
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
        knownMutableCollections[simple]?.let { return BuiltIn(simple, it) }
        return if ('.' in text) FqNames(setOf(FqName(text))) else SimpleName(simple)
    }

    // Every bound of the type (a flexible Java type has two) names a
    // configured class, as written (a type alias K2 expanded keeps the alias as
    // its abbreviation) or after type-alias expansion. Each bound is judged by
    // its own names: the platform type `(Mutable)List<T>!` of a Java
    // `java.util.List` return (`Collections.emptyList()`, `System.getenv()`)
    // has the read-only upper bound `List<T>?`, so it is not reported.
    private fun isConfiguredMutableType(type: ConeKotlinType, matchers: List<TypeMatcher>, session: FirSession): Boolean {
        val bounds = bounds(type, session) ?: return false
        return bounds.all { bound ->
            matchers.any { it.matches(bound.names) { isMutableBound(bound.expanded, session) } }
        }
    }

    private fun isMutableCollection(type: ConeKotlinType, session: FirSession): Boolean {
        val bounds = bounds(type, session) ?: return false
        return bounds.all { isMutableBound(it.expanded, session) }
    }

    // A `MutableIterable` or `MutableMap`, or a subtype. The class symbol comes
    // from the lookup tag, which is bound to local classes and anonymous
    // objects (`object : java.util.HashSet<String>() {}`), so no class id is
    // resolved through the symbol provider.
    private fun isMutableBound(bound: ConeClassLikeType, session: FirSession): Boolean {
        if (bound.lookupTag.classId in mutableRoots) return true
        val symbol = bound.lookupTag.toClassSymbol(session) ?: return false
        return lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
            .any { it.lookupTag.classId in mutableRoots }
    }

    /** One bound of a type: its class names as written, and its expansion. */
    private class Bound(val names: List<FqName>, val expanded: ConeClassLikeType)

    // The type's bounds (one, or a flexible type's lower and upper bound).
    private fun bounds(type: ConeKotlinType, session: FirSession): List<Bound>? {
        val bounds = listOf(type.lowerBoundIfFlexible(), type.upperBoundIfFlexible()).distinct()
        return bounds.map { bound ->
            val written = bound as? ConeClassLikeType ?: return null
            val expanded = written.fullyExpandedType(session) as? ConeClassLikeType ?: return null
            val alias = written.abbreviatedType as? ConeClassLikeType
            val names = listOfNotNull(alias, written, expanded).map { it.lookupTag.classId }.distinct()
            Bound(names.map { it.asSingleFqName() }, expanded)
        }
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
