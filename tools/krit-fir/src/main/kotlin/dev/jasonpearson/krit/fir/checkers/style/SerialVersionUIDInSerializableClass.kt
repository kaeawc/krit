package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.ClassNameFact
import dev.jasonpearson.krit.fir.FirRuleContext
import dev.jasonpearson.krit.fir.classNameFacts
import dev.jasonpearson.krit.fir.serializableReachedNames
import dev.jasonpearson.krit.fir.supertypeNames
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirRegularClassChecker
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.utils.isCompanion
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.providers.firProvider
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds
import java.util.WeakHashMap

/**
 * Port of the Go SerialVersionUIDInSerializableClass rule: a class that is
 * `java.io.Serializable` and declares no property named `serialVersionUID`
 * in its body or its companion object's body. Reported on the declaration's
 * first line (its modifier list, else its keyword), like Go, with Go's
 * message.
 *
 * Like Go, enum classes are exempt, any property named `serialVersionUID`
 * directly in the class body or in a companion object body counts (whatever
 * its type or modifiers), and a constructor property does not.
 *
 * The class is Serializable when `java.io.Serializable` is in its supertype
 * closure. Every Throwable is Serializable through `java.lang.Throwable`, so
 * the message is true of any exception without the field. Exceptions
 * conventionally carry none, and Go reports one only when its simple-name
 * resolver sees a Serializable supertype, so a class that is Serializable
 * only through Throwable is reported when Go's name-based evidence is there
 * (see [namesSerializable]): a supertype written as `Serializable` or
 * `Externalizable`, or written with the name of a source class that reaches
 * `Serializable` by supertype names. That keeps every Go finding on an
 * exception, including the ones Go reaches through a lookalike
 * `Serializable` or a same-named source class, and each of them is true.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`SerialVersionUIDInSerializableClass*.kt`):
 * - Go takes any direct supertype named `Serializable` or `Externalizable`,
 *   and any supertype whose simple name is also the name of a Serializable
 *   source class, so it reports a class that implements a lookalike
 *   interface, extends a same-named non-Serializable class, or names
 *   `Serializable` only in a type argument. Unless that class is a
 *   Throwable, it is not Serializable, so it is not reported here.
 * - Go reports an interface extending Serializable. Only classes are
 *   serialized; a `serialVersionUID` on an interface has no effect, so an
 *   interface is not missing one and is not reported here.
 * - Go compares the property's text, so a backticked `` `serialVersionUID` ``
 *   does not exempt the class there; it does here.
 * - Go follows supertypes only by simple name through Kotlin source classes,
 *   so it misses a class that is Serializable through a library or Java class
 *   other than Throwable (`ArrayList`, `Date`), an `Externalizable` source
 *   base class, a type alias, an import alias, a backticked name, or an
 *   interface delegation (`Serializable by impl`), and it never visits a
 *   named `object`. All of those are Serializable and lack the field, so they
 *   are reported here.
 */
internal object SerialVersionUIDInSerializableClass :
    FirRegularClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SerialVersionUIDInSerializableClass"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val regularClassCheckers = setOf(SerialVersionUIDInSerializableClass)
    }

    private val serializableClassId = ClassId(FqName("java.io"), Name.identifier("Serializable"))
    private val serialVersionUID = Name.identifier("serialVersionUID")
    private val throwableClassIds = setOf(
        StandardClassIds.Throwable,
        ClassId(FqName("java.lang"), Name.identifier("Throwable")),
    )
    private const val SERIALIZABLE = "Serializable"
    private const val EXTERNALIZABLE = "Externalizable"

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirRegularClass) {
        when (declaration.classKind) {
            ClassKind.CLASS -> Unit
            ClassKind.OBJECT -> if (declaration.isCompanion) return
            else -> return
        }
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (declaresSerialVersionUID(declaration)) return
        when (serializability(declaration, context.session)) {
            Serializability.NONE -> return
            Serializability.DECLARED -> Unit
            Serializability.THROWABLE -> if (!namesSerializable(declaration, context.session)) return
        }
        report(
            firstToken(source),
            "Serializable class '${declaration.name.asString()}' is missing serialVersionUID.",
        )
    }

    // A property named serialVersionUID directly in the class body or in a
    // companion object's body, as Go reads the class. Constructor properties
    // are instance state, not class-body declarations, so they do not count.
    // The class is a source declaration, so its direct declarations are
    // complete.
    @OptIn(DirectDeclarationsAccess::class)
    private fun declaresSerialVersionUID(declaration: FirRegularClass): Boolean {
        if (declaration.declarations.any { it.isBodyProperty() }) return true
        return declaration.declarations.any { member ->
            member is FirRegularClass && member.isCompanion && member.declarations.any { it.isBodyProperty() }
        }
    }

    private fun FirDeclaration.isBodyProperty(): Boolean =
        this is FirProperty && name == serialVersionUID && source?.kind is KtRealSourceElementKind

    private enum class Serializability {
        // Not java.io.Serializable.
        NONE,

        // java.io.Serializable only through java.lang.Throwable.
        THROWABLE,

        // java.io.Serializable through a supertype other than Throwable.
        DECLARED,
    }

    // Walks the supertype graph for java.io.Serializable, noting Throwable
    // without descending into it (see the class comment). Supertypes are read
    // from their own lookup tags, so no class id is resolved from a symbol
    // that may be local.
    private fun serializability(declaration: FirRegularClass, session: FirSession): Serializability {
        val visited = HashSet<FirClassSymbol<*>>()
        val pending = ArrayDeque<ConeClassLikeType>()
        fun enqueue(types: List<ConeKotlinType>) {
            for (type in types) {
                val expanded = type.fullyExpandedType(session).lowerBoundIfFlexible() as? ConeClassLikeType ?: continue
                pending.addLast(expanded)
            }
        }
        enqueue(declaration.superTypeRefs.map { it.coneType })
        var throwable = false
        while (pending.isNotEmpty()) {
            val type = pending.removeFirst()
            val classId = type.lookupTag.classId
            if (classId == serializableClassId) return Serializability.DECLARED
            if (classId in throwableClassIds) {
                throwable = true
                continue
            }
            val symbol = type.lookupTag.toClassSymbol(session) ?: continue
            if (!visited.add(symbol)) continue
            enqueue(symbol.resolvedSuperTypes)
        }
        return if (throwable) Serializability.THROWABLE else Serializability.NONE
    }

    // Go's evidence that an exception is Serializable, which is by name: a
    // supertype written as `Serializable` or `Externalizable`, whatever it
    // resolves to, or written with the simple name of a source class whose
    // supertype names reach `Serializable`. Go resolves a simple name to a
    // source class anywhere in the project, so a same-named class in another
    // package or nesting (`Resource.Error` for `kotlin.Error`) counts. Every
    // name in the supertype is taken, qualifiers and type arguments included,
    // as Go's index records them. Taking more names than Go can only add
    // findings on a Throwable, and those are true.
    private fun namesSerializable(declaration: FirRegularClass, session: FirSession): Boolean {
        val names = HashSet<String>()
        for (ref in declaration.superTypeRefs) supertypeNames(ref, names)
        if (SERIALIZABLE in names || EXTERNALIZABLE in names) return true
        val serializableNames = serializableSourceClassNames(session)
        return names.any { it in serializableNames }
    }

    // Per compile session, keyed also by the external facts the compile was given (identity): a slice supplies
    // the recorded facts of the files it does not compile from source, a full compile supplies none.
    private class CachedNames(val external: List<ClassNameFact>?, val names: Set<String>)

    private val serializableNamesBySession = WeakHashMap<FirSession, CachedNames>()

    // Simple names of the Kotlin classes of the module that reach `Serializable` by supertype names, the way Go's
    // resolver follows them: a class whose supertypes name `Serializable`, or name another such class. The facts
    // come from the source files of this compilation plus, in a slice, the recorded facts of the module's other
    // files ([FirRuleCompileContext.externalClassNameFacts]); a full compile holds every Kotlin source of the
    // project, as Go's index does.
    private fun serializableSourceClassNames(session: FirSession): Set<String> {
        val external = FirRuleContext.current()?.externalClassNameFacts
        return synchronized(serializableNamesBySession) {
            val cached = serializableNamesBySession[session]
            if (cached != null && cached.external === external) return@synchronized cached.names
            computeSerializableSourceClassNames(session, external).also { serializableNamesBySession[session] = CachedNames(external, it) }
        }
    }

    private fun computeSerializableSourceClassNames(session: FirSession, external: List<ClassNameFact>?): Set<String> {
        val facts = ArrayList<ClassNameFact>()
        external?.let { facts += it }
        val provider = session.firProvider
        for (pkg in provider.symbolProvider.symbolNamesProvider.getPackageNames().orEmpty()) {
            for (file in provider.getFirFilesByPackage(FqName(pkg))) facts += classNameFacts(file)
        }
        return serializableReachedNames(facts)
    }

    // Go reports the declaration's first line: its modifier list (annotations
    // included) when present, else its keyword. A leading KDoc belongs to the
    // declaration in the Kotlin tree but not in Go's, so skip comments.
    private fun firstToken(source: KtSourceElement): KtSourceElement {
        val first = lightChildren(source, source.lighterASTNode).firstOrNull {
            it.tokenType != KtTokens.WHITE_SPACE && it.tokenType !in KtTokens.COMMENTS
        } ?: return source
        return lightSourceOf(first, source)
    }
}
