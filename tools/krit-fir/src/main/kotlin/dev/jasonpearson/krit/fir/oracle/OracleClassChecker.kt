package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.descriptors.Visibility
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirClassChecker
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.declarations.FirEnumEntry
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.FirValueParameter
import org.jetbrains.kotlin.fir.declarations.utils.isData
import org.jetbrains.kotlin.fir.declarations.utils.modality
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeKotlinTypeProjection
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.FirTypeRef
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.renderReadable

/**
 * FIR `FirClassChecker` that projects each visited [FirRegularClass]
 * into a [ClassPayload] and pushes it onto the active [OracleCollector],
 * if any. Runs unconditionally during compilation but gates itself on
 * `OracleCollectorRegistry.current() != null`, so non-oracle paths
 * (e.g. the diagnostic-only `check()` command) pay only a thread-local
 * lookup per visited class.
 *
 * Covered surface: `fqn`, `kind`, `supertypes`, `visibility`, four
 * modality flags, `typeParameters`, and class members
 * (functions / properties / constructors / enum entries with name,
 * kind, returnType, nullable, visibility, override / abstract flags,
 * and parameters). Annotations and `jarPath` stay empty until their
 * respective projection passes land.
 */
internal object OracleClassChecker : FirClassChecker(MppCheckerKind.Common) {

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirClass) {
        // `reporter` is unused — this checker collects structured data
        // for the oracle response instead of emitting diagnostics.
        val collector = OracleCollectorRegistry.current() ?: return
        val regular = declaration as? FirRegularClass ?: return

        // `containingFileSymbol.fir.sourceFile.path` is the standard way
        // to recover the absolute path of the currently-checked file.
        // The `.fir` accessor is `SymbolInternals`-gated — annotated
        // above. No public alternative exists in K2 today.
        val filePath = context.containingFileSymbol?.fir?.sourceFile?.path ?: return

        collector.setPackage(filePath, regular.symbol.classId.packageFqName.asString())
        collector.addClass(filePath, regular.toClassPayload())
    }

    @OptIn(DirectDeclarationsAccess::class)
    private fun FirRegularClass.toClassPayload(): ClassPayload = ClassPayload(
        fqn = symbol.classId.asFqNameString(),
        kind = classKind.toWireString(),
        supertypes = superTypeRefs.mapNotNull { ref ->
            // Unresolved type refs (e.g. for sources with parse errors)
            // don't carry a `coneType`; skip them rather than crashing.
            (ref as? FirResolvedTypeRef)?.coneType?.renderReadable()
        },
        isSealed = modality == Modality.SEALED,
        isData = isData,
        isOpen = modality == Modality.OPEN,
        isAbstract = modality == Modality.ABSTRACT,
        visibility = visibility.toWireString(),
        typeParameters = typeParameters.map { it.symbol.name.asString() },
        members = declarations.mapNotNull { it.toMemberPayload() },
    )

    /**
     * Project one of a class's [declarations][FirRegularClass.declarations]
     * into a [MemberPayload]. Returns null for declaration kinds the
     * oracle does not surface today — nested classes are walked
     * separately by the per-file class pass, and synthetic / generated
     * declarations without a stable wire shape (e.g. compiler-synthesized
     * companion object fields) are skipped.
     */
    private fun FirDeclaration.toMemberPayload(): MemberPayload? = when (this) {
        is FirNamedFunction -> MemberPayload(
            name = name.asString(),
            kind = "function",
            returnType = returnTypeRef.renderType(),
            nullable = returnTypeRef.isNullable(),
            visibility = status.visibility.toWireString(),
            isOverride = status.isOverride,
            isAbstract = status.modality == Modality.ABSTRACT,
            params = valueParameters.map { it.toParamPayload() },
        )
        is FirProperty -> MemberPayload(
            name = name.asString(),
            kind = "property",
            returnType = returnTypeRef.renderType(),
            nullable = returnTypeRef.isNullable(),
            visibility = status.visibility.toWireString(),
            isOverride = status.isOverride,
            isAbstract = status.modality == Modality.ABSTRACT,
        )
        is FirConstructor -> MemberPayload(
            // `<init>` matches krit-types' canonical constructor name
            // and is also the JVM-level signature, so Go-side consumers
            // can pivot on the literal string.
            name = "<init>",
            kind = "constructor",
            returnType = returnTypeRef.renderType(),
            nullable = false,
            visibility = status.visibility.toWireString(),
            isOverride = false,
            isAbstract = false,
            params = valueParameters.map { it.toParamPayload() },
        )
        is FirEnumEntry -> MemberPayload(
            name = name.asString(),
            kind = "enum_entry",
            returnType = "",
            nullable = false,
            visibility = "public",
        )
        else -> null
    }

    private fun FirValueParameter.toParamPayload(): ParamPayload = ParamPayload(
        name = name.asString(),
        type = returnTypeRef.renderType(),
        nullable = returnTypeRef.isNullable(),
    )

    private fun FirTypeRef.renderType(): String =
        (this as? FirResolvedTypeRef)?.coneType?.renderFqn() ?: ""

    private fun FirTypeRef.isNullable(): Boolean =
        (this as? FirResolvedTypeRef)?.coneType?.isMarkedNullable == true

    /**
     * Render a [ConeKotlinType] as an FQN-qualified type string, matching
     * the krit-types wire format (e.g. `kotlin.String`, `kotlin.Int?`,
     * `kotlin.collections.List<kotlin.String>`). Non-class-like types
     * (type parameters, intersection types, flexible types) fall back to
     * the readable rendering since they don't carry a stable FQN.
     */
    private fun ConeKotlinType.renderFqn(): String {
        val classLike = this as? ConeClassLikeType ?: return renderReadable()
        val fqn = classLike.lookupTag.classId.asFqNameString()
        val args = typeArguments
            .takeIf { it.isNotEmpty() }
            ?.joinToString(", ", "<", ">") { projection ->
                (projection as? ConeKotlinTypeProjection)?.type?.renderFqn()
                    ?: projection.toString()
            }
            .orEmpty()
        val nullable = if (isMarkedNullable) "?" else ""
        return "$fqn$args$nullable"
    }

    private fun ClassKind.toWireString(): String = when (this) {
        ClassKind.CLASS -> "class"
        ClassKind.INTERFACE -> "interface"
        ClassKind.OBJECT -> "object"
        ClassKind.ENUM_CLASS -> "enum"
        ClassKind.ENUM_ENTRY -> "enum_entry"
        ClassKind.ANNOTATION_CLASS -> "annotation"
    }

    /**
     * Maps Kotlin's visibility values to the lowercased strings the
     * krit-types JSON shape uses. Unknown / module-local visibilities
     * fall back to `"public"` to match krit-types' default rendering.
     */
    private fun Visibility.toWireString(): String = when (this) {
        Visibilities.Public -> "public"
        Visibilities.Internal -> "internal"
        Visibilities.Private -> "private"
        Visibilities.PrivateToThis -> "private"
        Visibilities.Protected -> "protected"
        else -> "public"
    }
}
