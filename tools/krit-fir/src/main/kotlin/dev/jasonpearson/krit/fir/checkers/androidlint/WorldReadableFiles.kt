package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirQualifiedAccessExpressionChecker
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirResolvedImport
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.providers.symbolProvider
import org.jetbrains.kotlin.fir.scopes.getProperties
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirVariableSymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Flags the platform's deprecated world-readable file mode:
 * `android.content.Context.MODE_WORLD_READABLE` and
 * `android.os.ParcelFileDescriptor.MODE_WORLD_READABLE`.
 *
 * Like the Go rule, every reference to the constant is a finding, not only a
 * file-mode argument: a qualified or bare read (through any Context subclass,
 * such as `Activity.MODE_WORLD_READABLE`, a typealias, or an import alias), a
 * callable reference, and an import directive that names it (directly or
 * through a subclass). The finding sits on the MODE_WORLD_READABLE name, the
 * identifier Go reports, or on the import directive.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: Go reports every identifier spelled MODE_WORLD_READABLE that is
 *   not a property or variable name, so it also reports project declarations
 *   of that name (object and companion constants, enum entries, parameters and
 *   named-argument labels, locals and members that shadow the constant) and
 *   their imports. None of them is the platform's file mode, so FIR does not
 *   report them (WorldReadableFilesLookalike.kt,
 *   WorldReadableFilesLookalikeImport.kt, and a Java lookalike in
 *   WorldReadableFilesTest).
 * - Recall: an import alias of the constant (`import ...MODE_WORLD_READABLE
 *   as WR`) is still the constant where it is used; Go matches the name's
 *   text and misses it (WorldReadableFilesImports.kt).
 */
internal object WorldReadableFiles : FirQualifiedAccessExpressionChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "WorldReadableFiles"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val qualifiedAccessExpressionCheckers = setOf(WorldReadableFiles)
    }
    override val declarationCheckers = object : DeclarationCheckers() {
        override val fileCheckers = setOf(ImportChecker)
    }

    private const val MESSAGE = "MODE_WORLD_READABLE is insecure. Use more restrictive file permissions."
    private val NAME = Name.identifier("MODE_WORLD_READABLE")
    private val owners = setOf(
        ClassId(FqName("android.content"), Name.identifier("Context")),
        ClassId(FqName("android.os"), Name.identifier("ParcelFileDescriptor")),
    )
    private val constants = owners.mapTo(HashSet()) { CallableId(it, NAME) }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirQualifiedAccessExpression) {
        val symbol = expression.calleeReference.toResolvedCallableSymbol() as? FirVariableSymbol<*> ?: return
        if (symbol.name != NAME) return
        if (symbol.unwrapFakeOverrides().callableId !in constants) return
        report(expression.calleeReference.source ?: expression.source, MESSAGE)
    }

    private object ImportChecker : FirFileChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(declaration: FirFile) {
            for (import in declaration.imports) {
                val resolved = import as? FirResolvedImport ?: continue
                if (resolved.importedName != NAME) continue
                val owner = resolved.resolvedParentClassId ?: continue
                if (!importsPlatformConstant(owner)) continue
                report(import.source, MESSAGE)
            }
        }
    }

    // Whether `import <owner>.MODE_WORLD_READABLE` names a platform constant:
    // the owner declares it, or the owner's static scope (which holds the
    // statics a Java class inherits, as for `android.app.Activity`) resolves
    // the name to it. An import never names a local class, so the owner can be
    // looked up by class id.
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun importsPlatformConstant(owner: ClassId): Boolean {
        if (owner in owners) return true
        val klass = context.session.symbolProvider.getClassLikeSymbolByClassId(owner) as? FirRegularClassSymbol
            ?: return false
        val scope = klass.fir.scopeProvider.getStaticScope(klass.fir, context.session, context.scopeSession)
            ?: return false
        return scope.getProperties(NAME).any { it.unwrapFakeOverrides().callableId in constants }
    }
}
