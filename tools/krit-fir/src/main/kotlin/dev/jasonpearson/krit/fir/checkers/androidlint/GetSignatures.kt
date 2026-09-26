package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirPropertyAccessor
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.utils.isCompanion
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirBooleanOperatorExpression
import org.jetbrains.kotlin.fir.expressions.FirComparisonExpression
import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.FirNamedReference
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds
import org.jetbrains.kotlin.text
import kotlin.math.absoluteValue

// Flags a `getPackageInfo(...)` call whose flags argument requests
// PackageManager.GET_SIGNATURES, the deprecated signature query that can be
// spoofed (GET_SIGNING_CERTIFICATES replaces it on API 28+).
//
// Like the Go rule:
// - any call named getPackageInfo counts, whatever its receiver (a
//   PackageManager, a wrapper, an implicit receiver, a function-typed value
//   of that name); the flags argument is
//   the second value argument, or the one for a parameter named `flags`;
// - the flags argument asks for GET_SIGNATURES when it reads a variable or
//   constant named GET_SIGNATURES anywhere (in an `or` of flags, a
//   `PackageInfoFlags.of(...)`, a conversion); or when its value is an integer
//   literal with the 0x40 bit (`64`, `0x40`, `GET_META_DATA or 64`); or when
//   it is a local variable whose initializer asks for it;
// - an API guard exempts the call, exactly as Go spells it: an enclosing
//   function, class, object, or property annotated @RequiresApi or
//   @TargetApi (by the annotation's written name), or an enclosing `if` or
//   `when`, at any distance, that reads `Build.VERSION.SDK_INT` (written that
//   way) anywhere, in its condition or in either branch. A companion object,
//   a constructor, or an accessor's own annotation is not checked, and a
//   property's annotation covers its getter only when the getter starts on
//   the property's header line, where Go's tree nests it;
// - the finding is on the first line of the call expression (the receiver's
//   line for `pm.getPackageInfo(...)`), with Go's message.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Precision: a constant named GET_SIGNATURES whose value is a literal
//   without the 0x40 bit is not the Android flag and is not reported
//   (GetSignaturesLookalike). An integer literal only counts where it forms
//   the flags value (an operand of integer arithmetic or `or`, a branch
//   result, a `PackageInfoFlags.of` argument), not in a condition, a
//   comparison, or an argument of an unrelated call; Go counts any literal
//   inside the argument. A variable is followed through the declaration it
//   resolves to; Go takes the first same-named local declared earlier in the
//   function, in any scope, and also matches `x.flags` against a local
//   `flags` (GetSignaturesDivergence).
// - Recall: a variable is followed wherever the flags value reads it, through
//   any number of variables, member and top-level vals, and assignments to a
//   local var; Go only follows a local val or var named by the whole argument,
//   through its initializer, inside a named function. An import alias of
//   GET_SIGNATURES, a binary literal, and flags passed positionally after a
//   named argument (Go skips labeled arguments when counting to the second)
//   are reported (GetSignaturesRecall).
internal object GetSignatures : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "GetSignatures"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(GetSignatures)
    }

    private const val MESSAGE =
        "GET_SIGNATURES is deprecated and can be spoofed. Use GET_SIGNING_CERTIFICATES (API 28+) instead."

    private const val GET_SIGNATURES_BIT = 0x40L
    private val getPackageInfo = Name.identifier("getPackageInfo")
    private val getSignatures = Name.identifier("GET_SIGNATURES")
    private val sdkInt = Name.identifier("SDK_INT")
    private const val SDK_INT_SPELLING = "Build.VERSION.SDK_INT"
    private val flagsParameter = Name.identifier("flags")
    private val guardAnnotations = setOf("RequiresApi", "TargetApi")
    private val packageInfoFlagsOf = CallableId(
        ClassId(FqName("android.content.pm"), FqName("PackageManager.PackageInfoFlags"), isLocal = false),
        Name.identifier("of"),
    )
    private val integralClassIds = setOf(
        StandardClassIds.Int,
        StandardClassIds.Long,
        StandardClassIds.Short,
        StandardClassIds.Byte,
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (calledName(expression) != getPackageInfo) return
        val flags = flagsArgument(expression) ?: return
        if (apiGuarded(expression)) return
        if (!carriesFlag(flags, HashSet())) return
        report(expression.source, MESSAGE)
    }

    // The name the call is written with: for `getPackageInfo(a, b)` on a
    // property of function type, the property's name, not `invoke`.
    private fun calledName(call: FirFunctionCall): Name? =
        if (call is FirImplicitInvokeCall) {
            ((call.explicitReceiver as? FirQualifiedAccessExpression)?.calleeReference as? FirNamedReference)?.name
        } else {
            call.calleeReference.name
        }

    // The argument for the second value parameter, or else for a parameter
    // named `flags`, the way Go picks the second positional argument or the
    // one labeled `flags`.
    private fun flagsArgument(call: FirFunctionCall): FirExpression? {
        val mapping = call.resolvedArgumentMapping ?: return null
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return null
        val second = callee.valueParameterSymbols.getOrNull(1)
        mapping.entries.firstOrNull { it.value.symbol == second }?.let { return it.key }
        return mapping.entries.firstOrNull { it.value.name == flagsParameter }?.key
    }

    // Go's apiGuardedByVersionCheckFlat, over the enclosing elements.
    context(context: CheckerContext)
    private fun apiGuarded(call: FirFunctionCall): Boolean {
        val path = context.containingElements
        return path.withIndex().any { (index, element) ->
            element !== call && when (element) {
                is FirNamedFunction -> hasGuardAnnotation(element.annotations)
                // Go's class_declaration and object_declaration; a companion
                // object is a separate node Go does not check.
                is FirRegularClass -> !element.isCompanion && hasGuardAnnotation(element.annotations)
                is FirProperty -> {
                    val accessor = path.getOrNull(index + 1) as? FirPropertyAccessor
                    (accessor == null || accessorInsideDeclaration(element, accessor)) &&
                        hasGuardAnnotation(propertyAnnotations(element))
                }
                is FirWhenExpression -> isIfOrWhen(element) && readsSdkInt(element)
                else -> false
            }
        }
    }

    // Go's tree nests an accessor in the property declaration only when it
    // starts on the line the property's header ends on
    // (`val x: T get() = ...`); an accessor on a line of its own is a
    // sibling of the declaration, outside its annotations' reach.
    private fun accessorInsideDeclaration(property: FirProperty, accessor: FirPropertyAccessor): Boolean {
        val propertySource = property.source ?: return true
        val accessorSource = accessor.source?.takeIf { it.kind == KtRealSourceElementKind } ?: return true
        val text = propertySource.text ?: return true
        val offset = accessorSource.startOffset - propertySource.startOffset
        if (offset !in 0..text.length) return true
        return '\n' !in text.subSequence(0, offset).takeLastWhile { it.isWhitespace() }
    }

    // The annotations written on a property: FIR moves one that only applies
    // to a field onto the backing field, and a use-site targeted one
    // (`@get:RequiresApi`) onto its accessor. An accessor's own annotation
    // (`@RequiresApi get() = ...`) is not the property's.
    private fun propertyAnnotations(property: FirProperty): List<FirAnnotation> =
        property.annotations +
            property.backingField?.annotations.orEmpty() +
            listOfNotNull(property.getter, property.setter)
                .flatMap { it.annotations }
                .filter { it.useSiteTarget != null }

    // Go matches the annotation's written final name segment.
    private fun hasGuardAnnotation(annotations: List<FirAnnotation>): Boolean =
        annotations.any { annotation ->
            val written = annotation.annotationTypeRef.source?.text?.toString() ?: return@any false
            written.filterNot { it.isWhitespace() }.substringBefore('<').substringAfterLast('.') in guardAnnotations
        }

    private fun isIfOrWhen(expression: FirWhenExpression): Boolean {
        val source = expression.source ?: return false
        if (source.kind != KtRealSourceElementKind) return false
        return source.elementType == KtNodeTypes.IF || source.elementType == KtNodeTypes.WHEN
    }

    // Go looks for a navigation chain starting `Build.VERSION.SDK_INT`
    // anywhere inside the if or when, by spelling.
    private fun readsSdkInt(expression: FirWhenExpression): Boolean {
        var found = false
        expression.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirPropertyAccessExpression && element.calleeReference.name == sdkInt) {
                    val text = element.source?.text?.toString()?.filterNot { it.isWhitespace() || it == '?' }
                    if (text == SDK_INT_SPELLING) {
                        found = true
                        return
                    }
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    // Whether the flags value [element] may include GET_SIGNATURES. [seen]
    // holds the variables being followed, so a cycle ends the chain.
    context(context: CheckerContext)
    private fun carriesFlag(element: FirElement, seen: MutableSet<FirBasedSymbol<*>>): Boolean = when (element) {
        is FirWrappedArgumentExpression -> carriesFlag(element.expression, seen)
        is FirSmartCastExpression -> carriesFlag(element.originalExpression, seen)
        is FirLiteralExpression -> literalHasFlag(element)
        is FirFunctionCall ->
            if (isValueOperator(element)) {
                listOfNotNull(element.explicitReceiver).plus(element.argumentList.arguments).any { carriesFlag(it, seen) }
            } else {
                mentionsGetSignatures(element)
            }
        is FirQualifiedAccessExpression -> {
            val symbol = element.calleeReference.toResolvedCallableSymbol()
            when {
                symbol != null && isGetSignatures(symbol) -> true
                element.explicitReceiver?.let(::mentionsGetSignatures) == true -> true
                symbol is FirPropertySymbol -> variableCarriesFlag(symbol.unwrapFakeOverrides(), seen)
                else -> false
            }
        }
        is FirWhenExpression ->
            element.subjectVariable?.let(::mentionsGetSignatures) == true ||
                element.branches.any { mentionsGetSignatures(it.condition) || carriesFlag(it.result, seen) }
        is FirBlock -> {
            val statements = element.statements
            statements.dropLast(1).any(::mentionsGetSignatures) ||
                statements.lastOrNull()?.let { carriesFlag(it, seen) } == true
        }
        is FirTypeOperatorCall ->
            if (element.operation == FirOperation.AS || element.operation == FirOperation.SAFE_AS) {
                element.argumentList.arguments.any { carriesFlag(it, seen) }
            } else {
                mentionsGetSignatures(element)
            }
        is FirComparisonExpression,
        is FirEqualityOperatorCall,
        is FirBooleanOperatorExpression,
        is FirStringConcatenationCall,
        is FirAnonymousFunctionExpression,
        is FirVariableAssignment,
        -> mentionsGetSignatures(element)
        // Elvis, `!!`, try, safe calls: the value passes through.
        else -> directChildren(element).any { carriesFlag(it, seen) }
    }

    // An integer operator or conversion (`or`, `plus`, `toLong`, ...) and
    // PackageInfoFlags.of pass their operands into the flags value.
    private fun isValueOperator(call: FirFunctionCall): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        return callee.callableId == packageInfoFlagsOf || callee.callableId.classId in integralClassIds
    }

    // Go reads the literal's digits, so a negated literal counts by its
    // magnitude.
    private fun literalHasFlag(literal: FirLiteralExpression): Boolean {
        val value = when (val raw = literal.value) {
            is Int, is Long, is Short, is Byte -> (raw as Number).toLong()
            else -> return false
        }
        return value.absoluteValue and GET_SIGNATURES_BIT != 0L
    }

    // A variable or constant named GET_SIGNATURES, unless its declared value
    // is a literal that is not the flag (a project constant that merely
    // shares the name).
    @OptIn(SymbolInternals::class)
    private fun isGetSignatures(symbol: FirCallableSymbol<*>): Boolean {
        if (symbol.name != getSignatures) return false
        val initializer = when (val original = symbol.unwrapFakeOverrides()) {
            is FirPropertySymbol -> original.resolvedInitializer
            is FirFieldSymbol -> original.fir.initializer
            else -> null
        }
        return initializer !is FirLiteralExpression || literalHasFlag(initializer)
    }

    // A Kotlin property whose value may include the flag: its initializer,
    // and for a local var every assignment to it. A property with a custom
    // getter or a delegate has no known value.
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun variableCarriesFlag(symbol: FirPropertySymbol, seen: MutableSet<FirBasedSymbol<*>>): Boolean {
        if (!seen.add(symbol)) return false
        if (symbol.hasDelegate) return false
        val getter = symbol.getterSymbol
        if (getter != null && !getter.isDefault) return false
        if (!symbol.isLocal && !symbol.isVal) return false
        symbol.resolvedInitializer?.let { if (carriesFlag(it, seen)) return true }
        if (!symbol.isLocal || symbol.isVal) return false
        // A local var's whole scope lies in the outermost enclosing
        // non-class declaration.
        val root = context.containingDeclarations.firstOrNull { it !is FirClassLikeSymbol<*> && it !is FirFileSymbol }
            ?: return false
        var found = false
        root.fir.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirVariableAssignment && assignedSymbol(element) == symbol &&
                    carriesFlag(element.rValue, seen)
                ) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    // `x = y` assigns x; `x += y` assigns x through a desugared reference.
    private fun assignedSymbol(assignment: FirVariableAssignment): FirBasedSymbol<*>? {
        val lValue = assignment.lValue
        val target = if (lValue is FirDesugaredAssignmentValueReferenceExpression) lValue.expressionRef.value else lValue
        return (target as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
    }

    // Whether [element] reads GET_SIGNATURES anywhere.
    private fun mentionsGetSignatures(element: FirElement): Boolean {
        var found = false
        element.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirQualifiedAccessExpression) {
                    val symbol = element.calleeReference.toResolvedCallableSymbol()
                    if (symbol != null && isGetSignatures(symbol)) {
                        found = true
                        return
                    }
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun directChildren(element: FirElement): List<FirElement> {
        val children = ArrayList<FirElement>()
        element.acceptChildren(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                children += element
            }
        })
        return children
    }
}
