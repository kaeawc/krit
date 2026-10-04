package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousInitializer
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirPropertyAccessor
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.utils.isCompanion
import org.jetbrains.kotlin.fir.declarations.utils.isConst
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
import org.jetbrains.kotlin.fir.expressions.FirLoop
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWhenBranch
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.FirNamedReference
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
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

// Flags a `getPackageInfo(...)` call whose flags argument requests
// PackageManager.GET_SIGNATURES, the deprecated signature query that can be
// spoofed (GET_SIGNING_CERTIFICATES replaces it on API 28+).
//
// Like the Go rule:
// - any call named getPackageInfo counts, whatever its receiver (a
//   PackageManager, a wrapper, an implicit receiver, a function-typed value
//   of that name); the flags are the second unlabeled value argument (Go's
//   pick), the argument for the second parameter, and the argument for a
//   parameter named `flags`, and any of them may ask for GET_SIGNATURES;
// - the flags ask for GET_SIGNATURES when they read a variable or constant
//   named GET_SIGNATURES anywhere (in an `or` of flags, a
//   `PackageInfoFlags.of(...)`, a conversion, a lambda); or when their value
//   is an integer literal with the 0x40 bit (`64`, `0x40`,
//   `GET_META_DATA or 64`), also as a lambda's result (`run { 64 }`), the
//   receiver of a call that returns it (`64.also { }`, `coerceAtLeast`), an
//   operand of `maxOf`/`minOf`, or anywhere in a Kotlin library call outside
//   its lambdas and comparisons (`listOf(64).first()`); or when they read a
//   local variable whose value asks for it;
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
// - Precision: flags whose value lacks the 0x40 bit are not reported. A
//   constant expression is judged by its Int value (`-65`, `96 and 0x3F`, a
//   project constant named GET_SIGNATURES that is 1), and `and x.inv()` masks
//   the flag out (GetSignaturesDivergence, GetSignaturesLookalike). An integer
//   literal only counts where it may form the flags value, not in a condition,
//   a comparison, a lambda's statements, or an argument of a project
//   function; Go counts any literal inside the argument. A variable is followed through the declaration it
//   resolves to, and a local var through the assignments that may reach the
//   read; Go takes the initializer of the first same-named local declared
//   earlier in the function, in any scope, and also matches `x.flags` against
//   a local `flags` (GetSignaturesDivergence).
// - Recall: a variable is followed wherever the flags value reads it, through
//   any number of variables, member and top-level vals, and assignments to a
//   local var, in any enclosing scope; Go only follows a local val or var
//   named by the whole argument, through its initializer, inside the nearest
//   named function. An import alias of GET_SIGNATURES, a binary literal, a
//   constant expression with the bit (`-1`, `0x20 + 0x20`,
//   `GET_META_DATA.inv()`), flags passed
//   positionally after a named argument or labeled `flags` after a second
//   unlabeled argument, a parenthesized callee, and flags from a trailing
//   lambda are reported (GetSignaturesRecall).
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

    // Calls that return their receiver, or a value bounded by their operands.
    private val kotlin = FqName("kotlin")
    private val kotlinRanges = FqName("kotlin.ranges")
    private val receiverPassThrough = setOf(
        CallableId(kotlin, Name.identifier("also")),
        CallableId(kotlin, Name.identifier("apply")),
        CallableId(kotlin, Name.identifier("takeIf")),
        CallableId(kotlin, Name.identifier("takeUnless")),
        CallableId(kotlinRanges, Name.identifier("coerceAtLeast")),
        CallableId(kotlinRanges, Name.identifier("coerceAtMost")),
        CallableId(kotlinRanges, Name.identifier("coerceIn")),
    )
    private val operandPassThrough = setOf(
        CallableId(kotlinRanges, Name.identifier("coerceAtLeast")),
        CallableId(kotlinRanges, Name.identifier("coerceAtMost")),
        CallableId(kotlinRanges, Name.identifier("coerceIn")),
        CallableId(FqName("kotlin.comparisons"), Name.identifier("maxOf")),
        CallableId(FqName("kotlin.comparisons"), Name.identifier("minOf")),
        CallableId(FqName("kotlin.math"), Name.identifier("max")),
        CallableId(FqName("kotlin.math"), Name.identifier("min")),
        CallableId(ClassId(FqName("java.lang"), Name.identifier("Math")), Name.identifier("max")),
        CallableId(ClassId(FqName("java.lang"), Name.identifier("Math")), Name.identifier("min")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (calledName(expression) != getPackageInfo) return
        val flags = flagsArguments(expression)
        if (flags.isEmpty()) return
        if (apiGuarded(expression)) return
        val seen = HashSet<Any>()
        if (flags.none { carriesFlag(it, seen) }) return
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

    // The arguments that may be the flags: Go's pick (the second unlabeled
    // value argument, or else the one labeled `flags`), plus the arguments
    // for the second parameter and for a parameter named `flags`.
    private fun flagsArguments(call: FirFunctionCall): List<FirExpression> {
        val candidates = ArrayList<FirExpression>()
        secondUnlabeledArgument(call)?.let(candidates::add)
        val mapping = call.resolvedArgumentMapping
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*>
        if (mapping != null && callee != null) {
            val second = callee.valueParameterSymbols.getOrNull(1)
            for ((argument, parameter) in mapping) {
                if ((second != null && parameter.symbol == second) || parameter.name == flagsParameter) {
                    candidates += argument
                }
            }
        }
        return candidates
    }

    // The second argument written inside the parentheses without a name, the
    // one Go takes; a vararg's elements count one by one, a trailing lambda
    // not at all.
    private fun secondUnlabeledArgument(call: FirFunctionCall): FirExpression? {
        val arguments = call.argumentList.arguments.flatMap { argument ->
            (argument as? FirVarargArgumentsExpression)?.arguments ?: listOf(argument)
        }
        return arguments.firstOrNull { unlabeledArgumentIndex(it) == 1 }
    }

    // The position of [argument] among the unlabeled value arguments of its
    // call, or null when it is labeled or not inside the parentheses.
    private fun unlabeledArgumentIndex(argument: FirExpression): Int? {
        val source = argument.source?.takeIf { it.kind == KtRealSourceElementKind } ?: return null
        val tree = source.treeStructure
        var node = source.lighterASTNode
        // The argument itself may be a call; only an enclosing call, lambda or
        // argument list between it and its VALUE_ARGUMENT ends the climb.
        while (node.tokenType != KtNodeTypes.VALUE_ARGUMENT) {
            node = tree.getParent(node) ?: return null
            if (node.tokenType in argumentBoundaries) return null
        }
        val list = tree.getParent(node)?.takeIf { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST } ?: return null
        val unlabeled = lightChildren(source, list).filter {
            it.tokenType == KtNodeTypes.VALUE_ARGUMENT &&
                lightChildren(source, it).none { child -> child.tokenType == KtNodeTypes.VALUE_ARGUMENT_NAME }
        }
        val index = unlabeled.indexOfFirst { it.startOffset == node.startOffset && it.endOffset == node.endOffset }
        return index.takeIf { it >= 0 }
    }

    private val argumentBoundaries = setOf(
        KtNodeTypes.VALUE_ARGUMENT_LIST,
        KtNodeTypes.LAMBDA_ARGUMENT,
        KtNodeTypes.CALL_EXPRESSION,
    )

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
    // holds the variables (and local var reads) being followed, so a cycle
    // ends the chain.
    context(context: CheckerContext)
    private fun carriesFlag(element: FirElement, seen: MutableSet<Any>): Boolean {
        constantValue(element, 0)?.let { return it and GET_SIGNATURES_BIT != 0L }
        return when (element) {
            is FirWrappedArgumentExpression -> carriesFlag(element.expression, seen)
            is FirSmartCastExpression -> carriesFlag(element.originalExpression, seen)
            is FirLiteralExpression -> false
            is FirFunctionCall -> callCarriesFlag(element, seen)
            is FirQualifiedAccessExpression -> {
                val symbol = element.calleeReference.toResolvedCallableSymbol()
                when {
                    symbol != null && isGetSignatures(symbol) -> true
                    element.explicitReceiver?.let(::mentionsGetSignatures) == true -> true
                    symbol is FirPropertySymbol -> variableCarriesFlag(symbol.unwrapFakeOverrides(), element, seen)
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
            // Elvis, `!!`, try, safe calls, returns: the value passes through.
            else -> directChildren(element).any { carriesFlag(it, seen) }
        }
    }

    context(context: CheckerContext)
    private fun callCarriesFlag(call: FirFunctionCall, seen: MutableSet<Any>): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol
        val callableId = callee?.callableId
        val operands = listOfNotNull(call.explicitReceiver) + call.argumentList.arguments
        // An integer operator or conversion (`or`, `plus`, `toLong`, ...) and
        // PackageInfoFlags.of pass their operands into the flags value.
        if (callableId == packageInfoFlagsOf || callableId?.classId in integralClassIds) {
            return when (callee?.name?.asString()) {
                // `inv()` clears every bit its operand sets.
                "inv" -> false
                // `x and m` keeps only the bits of a constant mask.
                "and" -> operands.none(::clearsFlag) && operands.any { carriesFlag(it, seen) }
                else -> operands.any { carriesFlag(it, seen) }
            }
        }
        if (mentionsGetSignatures(call)) return true
        // `64.also { }`, `x.coerceAtLeast(64)`, `maxOf(64, x)`: the value may
        // be the receiver or an operand.
        if (callableId in receiverPassThrough && call.explicitReceiver?.let { carriesFlag(it, seen) } == true) return true
        if (callableId in operandPassThrough && flatArguments(call).any { carriesFlag(it, seen) }) return true
        // `run { 64 }`, `x.let { it or 64 }`: a lambda's result may be the value.
        val lambdaCarries = flatArguments(call).any { argument ->
            val lambda = unwrapArgument(argument) as? FirAnonymousFunctionExpression ?: return@any false
            lambdaResults(lambda).any { carriesFlag(it, seen) }
        }
        if (lambdaCarries) return true
        // A Kotlin library call (`listOf(64).first()`) may hand back any value
        // it is given; like Go, a literal with the bit in it counts, except in
        // a lambda's statements or a comparison. A project function's own
        // inputs are not taken for the flags.
        return callableId != null && isKotlinLibrary(callableId.packageName) && containsFlagLiteral(call)
    }

    private fun isKotlinLibrary(packageName: FqName): Boolean =
        packageName == kotlin || packageName.asString().startsWith("kotlin.")

    private fun containsFlagLiteral(element: FirElement): Boolean {
        var found = false
        element.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                // A lambda gives back only its results (followed above), and a
                // comparison only a Boolean.
                if (element is FirAnonymousFunctionExpression || element is FirComparisonExpression ||
                    element is FirEqualityOperatorCall
                ) {
                    return
                }
                if (element is FirLiteralExpression && constantValue(element, 0)?.let { it and GET_SIGNATURES_BIT != 0L } == true) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    // A constant without the 0x40 bit.
    private fun clearsFlag(operand: FirExpression): Boolean =
        constantValue(operand, 0)?.let { it and GET_SIGNATURES_BIT == 0L } == true

    private fun flatArguments(call: FirFunctionCall): List<FirExpression> =
        call.argumentList.arguments.flatMap { (it as? FirVarargArgumentsExpression)?.arguments ?: listOf(it) }

    private fun unwrapArgument(argument: FirExpression): FirExpression =
        if (argument is FirWrappedArgumentExpression) unwrapArgument(argument.expression) else argument

    // The values a lambda returns: its last expression and any
    // `return@label value` aimed at it.
    private fun lambdaResults(lambda: FirAnonymousFunctionExpression): List<FirExpression> {
        val function = lambda.anonymousFunction
        val results = ArrayList<FirExpression>()
        function.body?.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirReturnExpression && element.target.labeledElement === function) {
                    results += element.result
                }
                element.acceptChildren(this)
            }
        })
        if (results.isEmpty()) {
            function.body?.statements?.lastOrNull()?.let { if (it is FirExpression) results += it }
        }
        return results
    }

    // The Int value of a constant flags expression: an integer literal, a
    // constant, or bitwise and additive operators and conversions of them.
    // Every folded operation keeps the low bits the same whatever the width.
    @OptIn(SymbolInternals::class)
    private fun constantValue(element: FirElement, depth: Int): Long? {
        if (depth > 32) return null
        return when (element) {
            is FirWrappedArgumentExpression -> constantValue(element.expression, depth + 1)
            is FirLiteralExpression -> when (val raw = element.value) {
                is Int, is Long, is Short, is Byte -> (raw as Number).toLong()
                else -> null
            }
            is FirFunctionCall -> {
                val callee = element.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return null
                if (callee.callableId.classId !in integralClassIds) return null
                val receiver = element.explicitReceiver?.let { constantValue(it, depth + 1) } ?: return null
                val arguments = element.argumentList.arguments.map { constantValue(it, depth + 1) ?: return null }
                val operand = arguments.singleOrNull()
                when (callee.name.asString()) {
                    "or" -> operand?.let { receiver or it }
                    "and" -> operand?.let { receiver and it }
                    "xor" -> operand?.let { receiver xor it }
                    "plus" -> operand?.let { receiver + it }
                    "minus" -> operand?.let { receiver - it }
                    "times" -> operand?.let { receiver * it }
                    "inv" -> if (arguments.isEmpty()) receiver.inv() else null
                    "unaryMinus" -> if (arguments.isEmpty()) -receiver else null
                    "unaryPlus", "toInt", "toLong", "toShort", "toByte" -> if (arguments.isEmpty()) receiver else null
                    else -> null
                }
            }
            is FirQualifiedAccessExpression -> {
                val initializer = when (val symbol = element.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides()) {
                    is FirPropertySymbol -> if (symbol.isConst) symbol.resolvedInitializer else null
                    is FirFieldSymbol -> if (symbol.fir.isVal) symbol.fir.initializer else null
                    else -> null
                }
                initializer?.let { constantValue(it, depth + 1) }
            }
            else -> null
        }
    }

    // A variable or constant named GET_SIGNATURES, unless its declared value
    // is a constant that is not the flag (a project constant that merely
    // shares the name).
    @OptIn(SymbolInternals::class)
    private fun isGetSignatures(symbol: FirCallableSymbol<*>): Boolean {
        if (symbol.name != getSignatures) return false
        val initializer = when (val original = symbol.unwrapFakeOverrides()) {
            is FirPropertySymbol -> original.resolvedInitializer
            is FirFieldSymbol -> original.fir.initializer
            else -> null
        }
        val value = initializer?.let { constantValue(it, 0) } ?: return true
        return value and GET_SIGNATURES_BIT != 0L
    }

    // A Kotlin property whose value may include the flag: its initializer,
    // and for a local var the assignments that may reach [read]. A property
    // with a custom getter or a delegate has no known value.
    context(context: CheckerContext)
    private fun variableCarriesFlag(symbol: FirPropertySymbol, read: FirElement, seen: MutableSet<Any>): Boolean {
        val localVar = symbol.isLocal && !symbol.isVal
        if (!seen.add(if (localVar) symbol to read else symbol)) return false
        if (symbol.hasDelegate) return false
        val getter = symbol.getterSymbol
        if (getter != null && !getter.isDefault) return false
        if (!symbol.isLocal && !symbol.isVal) return false
        if (!localVar) return symbol.resolvedInitializer?.let { carriesFlag(it, seen) } == true
        // A local var's whole scope lies in the outermost enclosing
        // non-class declaration.
        val root = context.containingDeclarations.firstOrNull { it !is FirClassLikeSymbol<*> && it !is FirFileSymbol }
            ?: return false
        @OptIn(SymbolInternals::class)
        val rootFir = root.fir
        val values = reachingValues(symbol, read, rootFir)
            ?: (listOfNotNull(symbol.resolvedInitializer) + assignedValues(symbol, rootFir))
        return values.any { carriesFlag(it, seen) }
    }

    // The values of the local var [symbol] that may reach [read]: walking back
    // from the read through the enclosing blocks, every assignment inside an
    // earlier statement, up to the declaration or an assignment statement,
    // which replaces everything before it. Null when a loop or a function,
    // class, or object boundary lies between the read and the declaration,
    // where a later assignment may reach the read too.
    private fun reachingValues(symbol: FirPropertySymbol, read: FirElement, root: FirElement): List<FirExpression>? {
        val path = pathTo(root, read) ?: return null
        val values = ArrayList<FirExpression>()
        for (index in path.size - 2 downTo 0) {
            val element = path[index]
            val child = path[index + 1]
            when (element) {
                is FirBlock -> {
                    val position = element.statements.indexOfFirst { it === child }
                    if (position < 0) return null
                    for (statement in element.statements.subList(0, position).asReversed()) {
                        if (statement is FirProperty && statement.symbol == symbol) {
                            statement.initializer?.let(values::add)
                            return values
                        }
                        if (statement is FirVariableAssignment && assignedSymbol(statement) == symbol) {
                            values += statement.rValue
                            return values
                        }
                        values += assignedValues(symbol, statement)
                    }
                }
                is FirLoop, is FirFunction, is FirClass, is FirAnonymousInitializer -> return null
                // Only the subject and the conditions up to the read's branch
                // run before it; the other branches exclude it.
                is FirWhenExpression -> {
                    element.subjectVariable?.let { values += assignedValues(symbol, it) }
                    for (branch in element.branches) {
                        if (branch === child) break
                        values += assignedValues(symbol, branch.condition)
                    }
                }
                is FirWhenBranch -> if (child !== element.condition) values += assignedValues(symbol, element.condition)
                // Anything that starts before the read may run first.
                else -> {
                    val readStart = child.source?.startOffset
                    for (sibling in directChildren(element)) {
                        if (sibling === child) continue
                        val start = sibling.source?.startOffset
                        if (readStart == null || start == null || start < readStart) values += assignedValues(symbol, sibling)
                    }
                }
            }
        }
        return null
    }

    // The path of elements from [root] down to [target].
    private fun pathTo(root: FirElement, target: FirElement): List<FirElement>? {
        val stack = ArrayList<FirElement>()
        var result: List<FirElement>? = null
        root.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (result != null) return
                stack += element
                if (element === target) result = stack.toList() else element.acceptChildren(this)
                stack.removeAt(stack.size - 1)
            }
        })
        return result
    }

    // The values assigned to [symbol] anywhere inside [scope].
    private fun assignedValues(symbol: FirPropertySymbol, scope: FirElement): List<FirExpression> {
        val values = ArrayList<FirExpression>()
        scope.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirVariableAssignment && assignedSymbol(element) == symbol) values += element.rValue
                element.acceptChildren(this)
            }
        })
        return values
    }

    // `x = y` assigns x; `x += y` assigns x through a desugared reference.
    private fun assignedSymbol(assignment: FirVariableAssignment): Any? {
        val lValue = assignment.lValue
        val target = if (lValue is FirDesugaredAssignmentValueReferenceExpression) lValue.expressionRef.value else lValue
        return (target as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
    }

    // Whether [element] reads GET_SIGNATURES anywhere, other than inverted
    // (`GET_SIGNATURES.inv()` is a mask that clears the flag).
    private fun mentionsGetSignatures(element: FirElement): Boolean {
        var found = false
        element.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirFunctionCall && isIntegralInv(element)) return
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

    private fun isIntegralInv(call: FirFunctionCall): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        return callee.name.asString() == "inv" && callee.callableId.classId in integralClassIds
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
