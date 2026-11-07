package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirRegularClassChecker
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.utils.isCompanion
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Port of the Go SerialVersionUIDInSerializableClass rule, which follows detekt:
 * a class or object that lists `Serializable` as a declared supertype must declare
 * `private const val serialVersionUID: Long` in its companion object (or, for
 * an object, in its own body). A missing property is reported on the class
 * name; a property with another signature is reported on the property. The
 * messages are detekt's.
 *
 * Like detekt, the check is on the text of the declared supertype entries: an
 * entry is a match only when its text is exactly `Serializable`. A qualified
 * `java.io.Serializable`, `Externalizable`, a delegation (`Serializable by impl`),
 * and any class that is Serializable only through a parent class, a library
 * class, or `Throwable` are not matches, so no exception is reported for lacking
 * the field unless it names `Serializable` itself.
 *
 * Enum classes and interfaces are exempt (a `serialVersionUID` has no effect
 * there). A property in the class body itself or a constructor property does
 * not count, as in detekt.
 *
 * Like Go and detekt, the property is matched by its name text, so a backticked
 * `` `serialVersionUID` `` does not count and the class is reported.
 */
internal object SerialVersionUIDInSerializableClass :
    FirRegularClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SerialVersionUIDInSerializableClass"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val regularClassCheckers = setOf(SerialVersionUIDInSerializableClass)
    }

    private val serialVersionUID = Name.identifier("serialVersionUID")
    private const val SERIALIZABLE = "Serializable"

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirRegularClass) {
        when (declaration.classKind) {
            ClassKind.CLASS -> Unit
            ClassKind.OBJECT -> if (declaration.isCompanion) return
            else -> return
        }
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (!declaresSerializable(source)) return
        // Like detekt, only a property in the companion object (or in the object itself) counts.
        val property = serialVersionUIDProperty(declaration)
        if (property != null) {
            if (!hasCorrectSignature(property)) {
                report(
                    property.source ?: source,
                    "The property `serialVersionUID` signature is not correct. " +
                        "`serialVersionUID` should be `private` and `constant` and its type should be `Long`",
                )
            }
            return
        }
        val kind = if (declaration.classKind == ClassKind.OBJECT) "object" else "class"
        report(
            nameToken(source),
            "The $kind ${declaration.name.asString()} implements the `Serializable` interface and should thus define a `serialVersionUID`.",
        )
    }

    // The property named serialVersionUID in the companion object of a class, or in the body of an object.
    @OptIn(DirectDeclarationsAccess::class)
    private fun serialVersionUIDProperty(declaration: FirRegularClass): FirProperty? {
        val scope = if (declaration.classKind == ClassKind.OBJECT) {
            declaration
        } else {
            declaration.declarations.firstOrNull { it is FirRegularClass && it.isCompanion } as? FirRegularClass
        } ?: return null
        return scope.declarations.firstOrNull { it.isBodyProperty() } as? FirProperty
    }

    // `private const val serialVersionUID: Long`.
    private fun hasCorrectSignature(property: FirProperty): Boolean =
        property.status.visibility == Visibilities.Private && property.status.isConst &&
            (property.returnTypeRef.coneType as? ConeClassLikeType)?.lookupTag?.classId == StandardClassIds.Long

    private fun FirDeclaration.isBodyProperty(): Boolean =
        this is FirProperty && name == serialVersionUID && source?.let { it.kind is KtRealSourceElementKind && !hasBacktickedName(it) } == true

    // detekt and Go compare the name text, so a backticked name is not `serialVersionUID`.
    private fun hasBacktickedName(source: KtSourceElement): Boolean =
        lightChildren(source, source.lighterASTNode)
            .firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
            ?.let { lightText(source, it).startsWith("`") } == true

    // detekt compares the text of each declared supertype entry to `Serializable`.
    private fun declaresSerializable(source: KtSourceElement): Boolean =
        lightChildren(source, source.lighterASTNode)
            .filter { it.tokenType == KtNodeTypes.SUPER_TYPE_LIST }
            .flatMap { lightChildren(source, it) }
            .any { lightText(source, it).trim() == SERIALIZABLE }

    // Report the class name, so the finding stays off a preceding annotation or modifier line.
    // A declaration without a name falls back to its first token.
    private fun nameToken(source: KtSourceElement): KtSourceElement {
        val children = lightChildren(source, source.lighterASTNode)
        val first = children.firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
            ?: children.firstOrNull { it.tokenType != KtTokens.WHITE_SPACE && it.tokenType !in KtTokens.COMMENTS }
            ?: return source
        return lightSourceOf(first, source)
    }
}
