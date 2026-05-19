package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.descriptors.Visibility
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirClassChecker
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.utils.isData
import org.jetbrains.kotlin.fir.declarations.utils.modality
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.renderReadable

/**
 * FIR `FirClassChecker` that projects each visited [FirRegularClass]
 * into a [ClassPayload] and pushes it onto the active [OracleCollector],
 * if any. Runs unconditionally during compilation but gates itself on
 * `OracleCollectorRegistry.current() != null`, so non-oracle paths
 * (e.g. the diagnostic-only `check()` command) pay only a thread-local
 * lookup per visited class.
 *
 * The covered surface is intentionally narrow: `fqn`, `kind`,
 * `supertypes`, `visibility`, four modality flags (`isSealed`,
 * `isOpen`, `isAbstract`, `isData`), and `typeParameters`. Members,
 * annotations, and `jarPath` ride on later projection passes; the
 * `ClassPayload` defaults for those fields stay empty for now.
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
    )

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
