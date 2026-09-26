package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
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
 * closure, not counting the Serializable that every Throwable inherits from
 * `java.lang.Throwable`. Like Go, an exception class is reported only when
 * it (or a source base class) declares Serializable itself: exceptions are
 * Serializable as a platform detail and conventionally carry no
 * serialVersionUID.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`SerialVersionUIDInSerializableClass*.kt`):
 * - Go takes any direct supertype named `Serializable` or `Externalizable`,
 *   so it reports a class implementing a lookalike interface with that name.
 *   That class is not Serializable, so it is not reported here.
 * - Go reports an interface extending Serializable. Only classes are
 *   serialized; a `serialVersionUID` on an interface has no effect, so an
 *   interface is not missing one and is not reported here.
 * - Go follows supertypes only by simple name through source classes, so it
 *   misses a class that is Serializable through a library class other than
 *   Throwable (`ArrayList`, `Date`), an `Externalizable` source base class,
 *   a type alias, an import alias, or an interface delegation
 *   (`Serializable by impl`), and it never visits a named `object`. All of
 *   those are Serializable and lack the field, so they are reported here.
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
        if (!isSerializable(declaration, context.session)) return
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

    // Walks the supertype graph for java.io.Serializable without descending
    // into Throwable, which is Serializable only as a library detail (see the
    // class comment). Supertypes are read from their own lookup tags, so no
    // class id is resolved from a symbol that may be local.
    private fun isSerializable(declaration: FirRegularClass, session: FirSession): Boolean {
        val visited = HashSet<FirClassSymbol<*>>()
        val pending = ArrayDeque<ConeClassLikeType>()
        fun enqueue(types: List<ConeKotlinType>) {
            for (type in types) {
                val expanded = type.fullyExpandedType(session).lowerBoundIfFlexible() as? ConeClassLikeType ?: continue
                pending.addLast(expanded)
            }
        }
        enqueue(declaration.superTypeRefs.map { it.coneType })
        while (pending.isNotEmpty()) {
            val type = pending.removeFirst()
            val classId = type.lookupTag.classId
            if (classId == serializableClassId) return true
            if (classId in throwableClassIds) continue
            val symbol = type.lookupTag.toClassSymbol(session) ?: continue
            if (!visited.add(symbol)) continue
            enqueue(symbol.resolvedSuperTypes)
        }
        return false
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
