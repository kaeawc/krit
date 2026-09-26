package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirQualifiedAccessExpressionChecker
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirResolvedImport
import org.jetbrains.kotlin.fir.declarations.processAllDeclaredCallables
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.providers.symbolProvider
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags every use of Android's `Context.MODE_WORLD_WRITEABLE` file mode,
// reported on the line of the constant's name. Mirrors the Go rule, which
// reports every identifier spelled MODE_WORLD_WRITEABLE or MODE_WORLD_WRITABLE
// except a property's declared name:
// - a read of the constant, qualified (`Context.MODE_WORLD_WRITEABLE`,
//   `Activity.MODE_WORLD_WRITEABLE`), imported, or inherited inside a Context
//   subclass, in any expression (an argument, an initializer, a template, an
//   `or` of flags), and a callable reference to it;
// - an import of the constant (`import android.content.Context.MODE_WORLD_WRITEABLE`),
//   reported on the import line as Go reports the identifier there;
// - a read (or import) of a project property named MODE_WORLD_WRITEABLE or
//   MODE_WORLD_WRITABLE whose declared value is world-writeable: its
//   initializer, delegate, or getter reads the Android constant (directly or
//   through other properties), or its initializer is an Int literal with the
//   world-writeable bit (0x2) set. The code then does use a world-writeable
//   mode, as the message says.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall: a read through an import alias (`import ...Context.MODE_WORLD_WRITEABLE as WW`)
//   is reported; Go only sees the name on the import line. So is a read in a
//   short string template (`"$MODE_WORLD_WRITEABLE"`), which Go's identifier
//   dispatch does not see (it does see the braced `${...}` form).
// - Precision: a project declaration that only shares the name is not
//   Android's constant: a property whose value is not world-writeable
//   (`const val MODE_WORLD_WRITEABLE = 0`), a parameter, an enum entry, a
//   function, a named argument, or an import of any of them. Go reports each
//   of these by name (a property's declared name excepted).
internal object WorldWriteableFiles : FirQualifiedAccessExpressionChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "WorldWriteableFiles"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val qualifiedAccessExpressionCheckers = setOf(WorldWriteableFiles)
    }
    override val declarationCheckers = object : DeclarationCheckers() {
        override val fileCheckers = setOf(Imports)
    }

    private const val MESSAGE = "MODE_WORLD_WRITEABLE is insecure. Use more restrictive file permissions."
    private val contextClassId = ClassId(FqName("android.content"), Name.identifier("Context"))
    private val androidName = Name.identifier("MODE_WORLD_WRITEABLE")
    private val goNames = setOf(androidName, Name.identifier("MODE_WORLD_WRITABLE"))
    private const val WORLD_WRITEABLE_BIT = 0x2L

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirQualifiedAccessExpression) {
        val symbol = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (!isWorldWriteable(symbol)) return
        report(expression.calleeReference.source ?: expression.source, MESSAGE)
    }

    // An import of the constant, or of a world-writeable project property
    // named like it, on the import line.
    private object Imports : FirFileChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(declaration: FirFile) {
            for (import in declaration.imports) {
                val resolved = import as? FirResolvedImport ?: continue
                if (resolved.isAllUnder) continue
                val name = resolved.importedName ?: continue
                if (name !in goNames) continue
                if (importsWorldWriteable(resolved, name)) WorldWriteableFiles.report(resolved.source, MESSAGE)
            }
        }
    }

    // The imported callable named [name]: a member of the resolved parent
    // class (an import names only top-level and nested classes, never a local
    // one, so the class id resolves), or a top-level property of the package.
    context(context: CheckerContext)
    private fun importsWorldWriteable(import: FirResolvedImport, name: Name): Boolean {
        val parent = import.resolvedParentClassId
        if (parent == null) {
            return context.session.symbolProvider.getTopLevelPropertySymbols(import.packageFqName, name)
                .any { isWorldWriteable(it) }
        }
        val owner = context.session.symbolProvider.getClassLikeSymbolByClassId(parent) as? FirClassSymbol<*>
            ?: return false
        if (name == androidName && isContextOrSubclass(owner)) return true
        var found = false
        owner.processAllDeclaredCallables(context.session) { callable ->
            if (!found && callable.name == name && isWorldWriteable(callable)) found = true
        }
        return found
    }

    context(context: CheckerContext)
    private fun isWorldWriteable(symbol: FirBasedSymbol<*>): Boolean {
        if (isAndroidConstant(symbol)) return true
        if (symbol !is FirPropertySymbol || symbol.name !in goNames) return false
        if (symbol.hasWorldWriteableBit()) return true
        return declaredValueReadsConstant(symbol, HashSet())
    }

    // Android's field, read through Context or any subclass of it (a Java
    // static is inherited: `Activity.MODE_WORLD_WRITEABLE`).
    context(context: CheckerContext)
    private fun isAndroidConstant(symbol: FirBasedSymbol<*>): Boolean {
        if (symbol !is FirCallableSymbol<*>) return false
        val original = symbol.unwrapFakeOverrides()
        if (original.name != androidName) return false
        if (original.callableId?.classId == contextClassId) return true
        if (original !is FirFieldSymbol) return false
        val owner = original.getContainingClassSymbol() as? FirClassSymbol<*> ?: return false
        return isContextOrSubclass(owner)
    }

    context(context: CheckerContext)
    private fun isContextOrSubclass(owner: FirClassSymbol<*>): Boolean =
        owner.classId == contextClassId ||
            lookupSuperTypes(owner, lookupInterfaces = false, deep = true, useSiteSession = context.session)
                .any { it.lookupTag.classId == contextClassId }

    // `const val MODE_WORLD_WRITEABLE = 2`.
    private fun FirPropertySymbol.hasWorldWriteableBit(): Boolean {
        val literal = resolvedInitializer as? FirLiteralExpression ?: return false
        val value = literal.value as? Number ?: return false
        if (value !is Int && value !is Long) return false
        return value.toLong() and WORLD_WRITEABLE_BIT != 0L
    }

    // The property's initializer, delegate, or getter reads the Android
    // constant, directly or through another property whose declared value
    // does (any name). [seen] stops cycles.
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun declaredValueReadsConstant(symbol: FirPropertySymbol, seen: MutableSet<FirPropertySymbol>): Boolean {
        if (!seen.add(symbol)) return false
        val values = listOfNotNull(
            symbol.resolvedInitializer,
            symbol.delegate,
            symbol.getterSymbol?.takeUnless { it.isDefault }?.fir?.body,
        )
        return values.any { readsConstant(it, seen) }
    }

    context(context: CheckerContext)
    private fun readsConstant(value: FirElement, seen: MutableSet<FirPropertySymbol>): Boolean {
        var found = false
        value.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirPropertyAccessExpression) {
                    val read = element.calleeReference.toResolvedCallableSymbol()
                    if (read != null && (isAndroidConstant(read) ||
                            (read is FirPropertySymbol && declaredValueReadsConstant(read, seen)))
                    ) {
                        found = true
                        return
                    }
                }
                element.acceptChildren(this)
            }
        })
        return found
    }
}
