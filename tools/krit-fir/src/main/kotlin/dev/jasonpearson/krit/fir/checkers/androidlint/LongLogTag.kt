package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.containingClassLookupTag
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.InlineStatus
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.expressions.unwrapSmartcastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirBackingFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text

// Flags an android.util.Log level call (`v`, `d`, `i`, `w`, `e`, `wtf`) whose
// tag is longer than 23 characters, the limit older Android releases enforce
// in Log.isLoggable. Like the Go rule, the tag is known when the first
// argument (or the property of a safe call, `screen?.tag`) is:
// - a string literal without interpolation, raw strings included; or
// - a reference to a property (member, top-level, companion, object, or
//   local; `val`, `const val`, or `var`) initialized with such a literal, or
//   with a reference to another such property (`val TAG = LogTags.TAG`); or
// - a reference to an abstract or open property with no literal of its own,
//   when an override declared in the same file has one: that is the tag
//   passed for instances of the subclass, and Go finds it by name.
// Anything else (a parameter, a call, a concatenation, an interpolated
// template) is left alone. `Log.println` and `Log.isLoggable` are not
// checked, as in Go.
//
// The finding sits on the first line of the call expression (the receiver's
// line for `Log.d(...)`), with Go's message; the tag in the message is the
// literal's source spelling between the quotes, as Go reads it.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall: the call is identified by resolution, so a fully qualified
//   `android.util.Log.d(...)`, an import alias or typealias of Log, and a
//   statically imported `d(...)` are reported (LongLogTagRecall). Go needs
//   the receiver to be spelled `Log`. A tag property initialized with a
//   parenthesized literal (LongLogTagRecall), declared in another file
//   (LongLogTagCrossFileTest), or reaching a literal through properties of
//   other names (LongLogTagRecallChain) is resolved too; Go reads only bare
//   literals in the calling file, under the referenced name.
// - Precision: a project or local class or object named Log is not
//   android.util.Log and has no tag limit (LongLogTagLookalike); Go only
//   filters it out when the Kotlin oracle is running. Go finds a referenced
//   tag by name alone, taking the first property of that name anywhere in
//   the file whose initializer is a literal; FIR reads the property the
//   reference resolves to, so a parameter, a shadowing local, a same-named
//   property in another class, or a property with a custom getter does not
//   borrow another declaration's literal (LongLogTagDivergence), and a tag
//   Go misses because an earlier same-named property is short is reported
//   (LongLogTagRecallByName, LongLogTagRecallChain). A custom getter other
//   than `get() = field` makes the value unknown. A local var holds the
//   value of the last assignment that runs before the call on every path,
//   and its value is unknown in a stored lambda once it is reassigned
//   anywhere; Go always reads the initializer (LongLogTagDivergence,
//   LongLogTagLocalVar). The
//   limit is counted in characters of the runtime value (UTF-16, as Android
//   counts it); Go counts UTF-8 bytes of the source spelling, so a short
//   non-ASCII tag or a tag spelled with escape sequences can exceed 23 for
//   Go only (LongLogTagDivergence).
internal object LongLogTag : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "LongLogTag"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(LongLogTag)
    }

    private const val MAX_TAG_LENGTH = 23

    private val logClassId = ClassId(FqName("android.util"), Name.identifier("Log"))
    private val levelNames = setOf("v", "d", "i", "w", "e", "wtf").map(Name::identifier).toSet()

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        val callableId = callee.callableId
        if (callableId.classId != logClassId || callableId.callableName !in levelNames) return
        // Every level overload takes the tag first.
        val tagParameter = callee.valueParameterSymbols.firstOrNull() ?: return
        val argument = expression.resolvedArgumentMapping
            ?.entries
            ?.firstOrNull { it.value.symbol == tagParameter }
            ?.key
            ?: return
        val literal = tagLiteral(argument, HashSet()) ?: return
        val value = literal.value as? String ?: return
        if (value.length <= MAX_TAG_LENGTH) return
        report(expression.source, "Log tag \"${spelling(literal, value)}\" exceeds the 23 character limit.")
    }

    // The literal that gives the tag its value: the argument itself, or the
    // literal the property it reads resolves to. FIR drops parentheses around
    // either one; an interpolated template is not a literal. A safe call
    // (`screen?.tag`) passes the property's value whenever it passes a tag.
    context(context: CheckerContext)
    private fun tagLiteral(argument: FirExpression, path: MutableSet<FirPropertySymbol>): FirLiteralExpression? {
        val expression = unwrap(argument)
        if (expression is FirLiteralExpression) return expression
        if (expression !is FirPropertyAccessExpression) return null
        val symbol = expression.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return null
        return propertyLiteral(symbol.unwrapFakeOverrides(), expression, path)
    }

    // The literal a property's value comes from. Its initializer may itself
    // read another property (`private val TAG = LogTags.TAG`), which is
    // followed in turn; [path] holds the properties being followed, so a
    // cycle ends the chain. An abstract or open property with no literal of
    // its own takes the literal of an override declared in this file, as Go
    // does when it looks the name up.
    context(context: CheckerContext)
    private fun propertyLiteral(
        symbol: FirPropertySymbol,
        use: FirElement,
        path: MutableSet<FirPropertySymbol>,
    ): FirLiteralExpression? {
        if (!path.add(symbol)) return null
        try {
            ownLiteral(symbol, use, path)?.let { return it }
            if (symbol.isLocal) return null
            val modality = symbol.resolvedStatus.modality
            if (modality != Modality.ABSTRACT && modality != Modality.OPEN) return null
            return overrideLiteral(symbol, path)
        } finally {
            path.remove(symbol)
        }
    }

    context(context: CheckerContext)
    private fun ownLiteral(
        symbol: FirPropertySymbol,
        use: FirElement,
        path: MutableSet<FirPropertySymbol>,
    ): FirLiteralExpression? {
        if (symbol.hasDelegate) return null
        val getter = symbol.getterSymbol
        if (getter != null && !getter.isDefault && !returnsField(getter)) return null
        val value = if (symbol.isLocal && !symbol.isVal) {
            localVarValue(symbol, use) ?: return null
        } else {
            symbol.resolvedInitializer ?: return null
        }
        return tagLiteral(value, path)
    }

    // The first override of [symbol] declared in this file, in source order,
    // whose tag is over the limit: the tag the call passes for instances of
    // that subclass. Subclasses are found through their own supertype lookup
    // tags, so no class id is resolved from a local or anonymous class.
    @OptIn(SymbolInternals::class, DirectDeclarationsAccess::class)
    context(context: CheckerContext)
    private fun overrideLiteral(symbol: FirPropertySymbol, path: MutableSet<FirPropertySymbol>): FirLiteralExpression? {
        val ownerId = symbol.containingClassLookupTag()?.classId ?: return null
        val file = context.containingFileSymbol?.fir ?: return null
        val overrides = ArrayList<FirProperty>()
        file.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirClass && element.symbol.classId != ownerId) {
                    val matching = element.declarations.filterIsInstance<FirProperty>().filter {
                        it.name == symbol.name && it.status.isOverride && it.receiverParameter == null
                    }
                    if (matching.isNotEmpty() && isSubclass(element.symbol, ownerId)) overrides += matching
                }
                element.acceptChildren(this)
            }
        })
        for (override in overrides) {
            val literal = propertyLiteral(override.symbol, override, path) ?: continue
            val value = literal.value as? String ?: continue
            if (value.length > MAX_TAG_LENGTH) return literal
        }
        return null
    }

    context(context: CheckerContext)
    private fun isSubclass(symbol: FirClassSymbol<*>, ownerId: ClassId): Boolean =
        lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = context.session)
            .any { it.classId == ownerId }

    // The expression whose value the local var [variable] holds where [use]
    // reads it. The value can be the initializer as long as no assignment
    // runs before the read on every path, so the tag is still reported when
    // the var is reassigned only after the call, on one branch, or later in
    // a loop body (the first pass reads the initializer). An assignment that
    // is a statement of a block enclosing the read, earlier in that block,
    // always runs first: the last such assignment gives the value, and a
    // compound one (`tag += x`) makes it unknown. A read inside a lambda that
    // is not inlined, a local function, or a local class may run at any
    // time, so there only an assignment inside that same body counts, and
    // any other assignment makes the value unknown. A local var's whole scope
    // lies in the outermost enclosing declaration, which is the one scanned.
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun localVarValue(variable: FirPropertySymbol, use: FirElement): FirExpression? {
        val root = context.containingDeclarations.firstOrNull { it !is FirClassLikeSymbol<*> && it !is FirFileSymbol }
            ?: return null
        val scan = LocalVarScan(variable, use)
        root.fir.accept(scan)
        if (scan.assignments.isEmpty()) return variable.resolvedInitializer
        val useAncestors = scan.useAncestors ?: return null
        val declarationAncestors = scan.declarationAncestors ?: return null
        val useOffset = use.source?.startOffset ?: return null
        // The innermost body around the read that may run at another time
        // than the declaration's own code.
        val deferred = useAncestors.indexOfLast { scope ->
            isDeferred(scope) && declarationAncestors.none { it === scope }
        }
        val dominating = scan.assignments.filter { (assignment, parent) ->
            val block = parent as? FirBlock ?: return@filter false
            val end = assignment.source?.endOffset ?: return@filter false
            useAncestors.indexOfFirst { it === block } > deferred && end <= useOffset
        }
        val last = dominating.maxByOrNull { (assignment, _) -> assignment.source?.startOffset ?: 0 }?.first
        if (last != null) {
            return if (last.lValue is FirDesugaredAssignmentValueReferenceExpression) null else last.rValue
        }
        return if (deferred >= 0) null else variable.resolvedInitializer
    }

    private fun isDeferred(element: FirElement): Boolean = when (element) {
        is FirAnonymousFunction -> element.inlineStatus != InlineStatus.Inline
        is FirFunction, is FirClass -> true
        else -> false
    }

    // Finds the ancestors of the read and of the var's declaration, and every
    // assignment to the var with the element that directly contains it.
    private class LocalVarScan(
        private val variable: FirPropertySymbol,
        private val use: FirElement,
    ) : FirVisitorVoid() {
        private val ancestors = ArrayList<FirElement>()
        var useAncestors: List<FirElement>? = null
        var declarationAncestors: List<FirElement>? = null
        val assignments = ArrayList<Pair<FirVariableAssignment, FirElement?>>()

        override fun visitElement(element: FirElement) {
            if (element === use) useAncestors = ancestors.toList()
            if (element is FirProperty && element.symbol == variable) declarationAncestors = ancestors.toList()
            if (element is FirVariableAssignment && assignedSymbol(element) == variable) {
                assignments += element to ancestors.lastOrNull()
            }
            ancestors += element
            element.acceptChildren(this)
            ancestors.removeAt(ancestors.lastIndex)
        }

        // `x = y` assigns x; `x += y` assigns x through a desugared reference.
        private fun assignedSymbol(assignment: FirVariableAssignment): FirBasedSymbol<*>? {
            val lValue = assignment.lValue
            val target = if (lValue is FirDesugaredAssignmentValueReferenceExpression) lValue.expressionRef.value else lValue
            return (target as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
        }
    }

    // A custom getter that only returns the backing field (`get() = field`)
    // still yields the initializer's value, so it keeps the finding.
    @OptIn(SymbolInternals::class)
    private fun returnsField(getter: FirFunctionSymbol<*>): Boolean {
        val statement = getter.fir.body?.statements?.singleOrNull()
        val result = if (statement is FirReturnExpression) statement.result else statement
        val access = result as? FirPropertyAccessExpression ?: return false
        return access.calleeReference.toResolvedCallableSymbol() is FirBackingFieldSymbol
    }

    private fun unwrap(argument: FirExpression): FirExpression {
        var current = argument
        while (true) {
            val next = when (current) {
                is FirWrappedArgumentExpression -> current.expression
                is FirSafeCallExpression -> current.selector as? FirExpression ?: return current
                else -> current.unwrapSmartcastExpression()
            }
            if (next === current) return current
            current = next
        }
    }

    // The literal's source text between its quotes (escape sequences
    // undecoded), which is how Go prints the tag; the runtime value when the
    // literal has no source, such as a constant from a library.
    private fun spelling(literal: FirLiteralExpression, value: String): String {
        val text = literal.source?.text?.toString()?.trim() ?: return value
        return when {
            text.length >= 6 && text.startsWith(RAW_QUOTE) && text.endsWith(RAW_QUOTE) ->
                text.substring(RAW_QUOTE.length, text.length - RAW_QUOTE.length)
            text.length >= 2 && text.startsWith(QUOTE) && text.endsWith(QUOTE) ->
                text.substring(QUOTE.length, text.length - QUOTE.length)
            else -> value
        }
    }

    private const val QUOTE = "\""
    private const val RAW_QUOTE = "\"\"\""
}
