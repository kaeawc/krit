package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.importedNameSource
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.KtSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirQualifiedAccessExpressionChecker
import org.jetbrains.kotlin.fir.analysis.checkers.unsubstitutedScope
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirResolvedImport
import org.jetbrains.kotlin.fir.declarations.FirValueParameter
import org.jetbrains.kotlin.fir.declarations.processAllDeclaredCallables
import org.jetbrains.kotlin.fir.declarations.utils.isOverride
import org.jetbrains.kotlin.fir.declarations.utils.modality
import org.jetbrains.kotlin.fir.expressions.FirCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.providers.symbolProvider
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirVariableSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

// Flags every use of Android's `Context.MODE_WORLD_WRITEABLE` file mode,
// reported on the line of the name. Mirrors the Go rule, which reports every
// identifier spelled MODE_WORLD_WRITEABLE or MODE_WORLD_WRITABLE except a
// property's declared name:
// - a read of the constant, qualified (`Context.MODE_WORLD_WRITEABLE`,
//   `Activity.MODE_WORLD_WRITEABLE`), imported, or inherited inside a Context
//   subclass, in any expression (an argument, an initializer, a template, an
//   `or` of flags), and a callable reference to it;
// - an import of the constant (`import android.content.Context.MODE_WORLD_WRITEABLE`),
//   reported on the import line as Go reports the identifier there;
// - a project variable named MODE_WORLD_WRITEABLE or MODE_WORLD_WRITABLE
//   (property, local, Java field, or parameter) whose value may be
//   world-writeable: each read and import of it, the parameter's name, the
//   assigned name of an assignment to it, and a named-argument label with
//   that name. Only a value that is provably not world-writeable drops these
//   findings (see [Value]): a const-evaluable value without the
//   world-writeable bit (0x2), or a parameter without a world-writeable
//   default. A value FIR cannot see (a var, an abstract or open property, a
//   function-call initializer, a binary property) is reported, as Go does.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall: a read through an import alias (`import ...MODE_WORLD_WRITEABLE as WW`,
//   of the Android constant or of a world-writeable project property) is
//   reported; Go only sees the name on the import line. So is a read in a
//   short string template (`"$MODE_WORLD_WRITEABLE"`), which Go's identifier
//   dispatch does not see (it does see the braced `${...}` form).
// - Precision: a declaration that only shares the name is not Android's
//   constant: a property or parameter default whose value is provably not
//   world-writeable (`const val MODE_WORLD_WRITEABLE = 0`, `1 shl 2`,
//   `Context.MODE_PRIVATE`), a parameter without a default, an enum entry, a
//   function, a named argument or assignment whose value is provably not
//   world-writeable, or an import of any of them. Go reports each of these by
//   name (a property's declared name excepted).
internal object WorldWriteableFiles : FirQualifiedAccessExpressionChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "WorldWriteableFiles"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val qualifiedAccessExpressionCheckers = setOf(WorldWriteableFiles)
        override val callCheckers = setOf(NamedArguments)
    }
    override val declarationCheckers = object : DeclarationCheckers() {
        override val fileCheckers = setOf(Imports)
        override val valueParameterCheckers = setOf(Parameters)
    }

    private const val MESSAGE = "MODE_WORLD_WRITEABLE is insecure. Use more restrictive file permissions."
    private val contextClassId = ClassId(FqName("android.content"), Name.identifier("Context"))
    private val androidName = Name.identifier("MODE_WORLD_WRITEABLE")
    private val goNames = setOf(androidName, Name.identifier("MODE_WORLD_WRITABLE"))
    private val goNameStrings = goNames.mapTo(HashSet()) { it.asString() }
    private const val WORLD_WRITEABLE_BIT = 0x2L
    private val integralClassIds = setOf(StandardClassIds.Int, StandardClassIds.Long, StandardClassIds.Short, StandardClassIds.Byte)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirQualifiedAccessExpression) {
        // Generated reads that point at a name reported elsewhere: a `val`
        // constructor parameter's property reads the parameter (reported by
        // [Parameters]), a delegated property's accessors reference the
        // property itself, a data class's generated members read its
        // properties, and a prefix increment reads the variable twice.
        if (isGeneratedRead(expression.source?.kind)) return
        val symbol = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val assignment = context.containingElements.lastOrNull { it !== expression } as? FirVariableAssignment
        if (assignment != null && assignment.lValue === expression) {
            if (isWorldWriteableAssignment(assignment, symbol)) report(expression.calleeReference.source ?: expression.source, MESSAGE)
            return
        }
        if (!isWorldWriteable(symbol)) return
        report(expression.calleeReference.source ?: expression.source, MESSAGE)
    }

    private fun isGeneratedRead(kind: KtSourceElementKind?): Boolean =
        kind == KtFakeSourceElementKind.PropertyFromParameter ||
            kind == KtFakeSourceElementKind.DelegatedPropertyAccessor ||
            kind == KtFakeSourceElementKind.DataClassGeneratedMembers ||
            kind is KtFakeSourceElementKind.DesugaredPrefixSecondGetReference

    // `MODE_WORLD_WRITEABLE = value`: Go reports the assigned name, so FIR
    // does unless the value written is provably not world-writeable. A
    // compound assignment or increment (`+=`, `++`) is desugared with a fake
    // source; its read of the variable is reported instead, once, as Go
    // reports the name once.
    context(context: CheckerContext)
    private fun isWorldWriteableAssignment(assignment: FirVariableAssignment, target: FirCallableSymbol<*>): Boolean {
        if (assignment.source?.kind is KtFakeSourceElementKind) return false
        if (isAndroidConstant(target)) return true
        if (target.unwrapFakeOverrides().name !in goNames || target !is FirVariableSymbol<*>) return false
        return !evaluate(assignment.rValue, HashSet()).isProvablyNotWorldWriteable()
    }

    // An import of the constant, or of a project variable named like it whose
    // value may be world-writeable, on the import line.
    private object Imports : FirFileChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(declaration: FirFile) {
            for (import in declaration.imports) {
                val resolved = import as? FirResolvedImport ?: continue
                if (resolved.isAllUnder) continue
                val name = resolved.importedName ?: continue
                if (name !in goNames) continue
                val source = resolved.source ?: continue
                if (importsWorldWriteable(resolved, name)) WorldWriteableFiles.report(importedNameSource(source), MESSAGE)
            }
        }
    }

    // A parameter named like the constant whose default may be
    // world-writeable: Go reports the parameter's name, and a caller that
    // omits the argument uses the default. Go skips a lambda's parameter
    // name (tree-sitter parses it as a variable declaration); its reads are
    // still reported.
    private object Parameters : FirDeclarationChecker<FirValueParameter>(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(declaration: FirValueParameter) {
            val source = declaration.source ?: return
            if (source.kind is KtFakeSourceElementKind) return
            if (declaration.name !in goNames) return
            if ((declaration.symbol.containingDeclarationSymbol as? FirAnonymousFunctionSymbol)?.isLambda == true) return
            if (parameterValue(declaration.symbol, HashSet()).isProvablyNotWorldWriteable()) return
            val name = lightChildren(source, source.lighterASTNode).firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
            WorldWriteableFiles.report(name?.let { lightSourceOf(it, source) } ?: source, MESSAGE)
        }
    }

    // `f(MODE_WORLD_WRITEABLE = value)`: Go reports the argument's label.
    private object NamedArguments : FirExpressionChecker<FirCall>(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirCall) {
            for (argument in expression.argumentList.arguments.flatMap { flattenVararg(it) }) {
                val label = namedArgumentLabel(argument) ?: continue
                val value = (argument as? FirWrappedArgumentExpression)?.expression ?: argument
                if (evaluate(value, HashSet()).isProvablyNotWorldWriteable()) continue
                WorldWriteableFiles.report(label, MESSAGE)
            }
        }

        private fun flattenVararg(argument: FirExpression): List<FirExpression> =
            (argument as? FirVarargArgumentsExpression)?.arguments ?: listOf(argument)

        // The `NAME` of a `NAME = value` argument when NAME is one of Go's
        // names, from the source tree (the argument may or may not keep its
        // named-argument wrapper after resolution).
        private fun namedArgumentLabel(argument: FirExpression): KtSourceElement? {
            val source = argument.source ?: return null
            if (source.kind is KtFakeSourceElementKind) return null
            val tree = source.treeStructure
            val node = source.lighterASTNode
            val valueArgument = if (node.tokenType == KtNodeTypes.VALUE_ARGUMENT) {
                node
            } else {
                tree.getParent(node)?.takeIf { it.tokenType == KtNodeTypes.VALUE_ARGUMENT } ?: return null
            }
            val label = lightChildren(source, valueArgument).firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_NAME }
                ?: return null
            if (tree.toString(label).toString().trim().removeSurrounding("`") !in goNameStrings) return null
            return lightSourceOf(label, source)
        }
    }

    // The imported callable named [name]: a member (declared or inherited) of
    // the resolved parent class (an import names only top-level and nested
    // classes, never a local one, so the class id resolves), or a top-level
    // property of the package.
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
        if (!found) {
            owner.unsubstitutedScope().processPropertiesByName(name) { property ->
                if (!found && isWorldWriteable(property)) found = true
            }
        }
        return found
    }

    // A read of Android's constant, or of a variable named like it whose value
    // may be world-writeable. A fake or substitution override (a member
    // inherited from a generic supertype) is unwrapped to its declaration.
    context(context: CheckerContext)
    private fun isWorldWriteable(symbol: FirBasedSymbol<*>): Boolean {
        if (symbol !is FirCallableSymbol<*>) return false
        if (isAndroidConstant(symbol)) return true
        val original = symbol.unwrapFakeOverrides()
        if (original.name !in goNames) return false
        val value = when (original) {
            is FirValueParameterSymbol -> parameterValue(original, HashSet())
            is FirPropertySymbol -> propertyValue(original, HashSet())
            is FirFieldSymbol -> fieldValue(original)
            else -> return false // an enum entry or a function: not a mode
        }
        return !value.isProvablyNotWorldWriteable()
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

    // What FIR knows about an Int value. Only [Known] without the
    // world-writeable bit, [NotAMode], and [CallerSupplied] are provably not
    // world-writeable; a read of Android's constant anywhere in the value
    // makes it [ReadsConstant], and anything FIR cannot evaluate is [Unknown].
    private sealed interface Value {
        data class Known(val bits: Long) : Value
        data object ReadsConstant : Value
        data object NotAMode : Value

        // A parameter without a default: the value comes from the caller,
        // whose own read of a world-writeable mode is reported where it is
        // written.
        data object CallerSupplied : Value
        data object Unknown : Value
    }

    private fun Value.isProvablyNotWorldWriteable(): Boolean = when (this) {
        is Value.Known -> bits and WORLD_WRITEABLE_BIT == 0L
        Value.NotAMode, Value.CallerSupplied -> true
        Value.ReadsConstant, Value.Unknown -> false
    }

    // A property's value, when its declaration pins it down: a final val
    // (not overridable) with an initializer or a single-expression getter.
    // A delegate or custom getter that reads Android's constant still counts.
    // [seen] holds the variables being evaluated, to stop cycles.
    context(context: CheckerContext)
    private fun propertyValue(symbol: FirPropertySymbol, seen: MutableSet<FirVariableSymbol<*>>): Value {
        val property = symbol.unwrapFakeOverrides()
        if (!seen.add(property)) return Value.Unknown
        try {
            return declaredPropertyValue(property, seen)
        } finally {
            seen.remove(property)
        }
    }

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun declaredPropertyValue(property: FirPropertySymbol, seen: MutableSet<FirVariableSymbol<*>>): Value {
        val getter = property.getterSymbol?.takeUnless { it.isDefault }
        val delegate = property.delegate
        val declared = when {
            getter != null -> {
                val body = getter.fir.body
                val single = (body?.statements?.singleOrNull() as? FirReturnExpression)?.result
                if (single != null) evaluate(single, seen) else if (body != null && readsConstant(body)) Value.ReadsConstant else Value.Unknown
            }
            delegate != null -> if (readsConstant(delegate)) Value.ReadsConstant else Value.Unknown
            else -> property.resolvedInitializer?.let { evaluate(it, seen) } ?: Value.Unknown
        }
        if (declared == Value.ReadsConstant) return declared
        // Another value can replace the declared one: a var is reassigned, and
        // an open or abstract member is overridden.
        if (property.isVar || isOverridable(property)) return Value.Unknown
        return declared
    }

    // An open or abstract member of a class that can have subclasses: an
    // interface, an open or abstract class, or an enum class (its entries).
    context(context: CheckerContext)
    private fun isOverridable(property: FirPropertySymbol): Boolean {
        if (property.modality == Modality.FINAL) return false
        val owner = property.getContainingClassSymbol() as? FirClassSymbol<*> ?: return false
        if (owner.classKind == ClassKind.OBJECT || owner.classKind == ClassKind.ENUM_ENTRY) return false
        return owner.classKind != ClassKind.CLASS || owner.modality != Modality.FINAL
    }

    // A parameter's value: its default, or the caller's. An override's
    // parameter inherits the overridden default, which FIR does not follow,
    // and a lambda's parameter is supplied by whatever invokes the lambda
    // (usually library code iterating over values), so both are unknown.
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun parameterValue(symbol: FirValueParameterSymbol, seen: MutableSet<FirVariableSymbol<*>>): Value {
        val default = symbol.fir.defaultValue
        if (default != null) {
            if (!seen.add(symbol)) return Value.Unknown
            try {
                return evaluate(default, seen)
            } finally {
                seen.remove(symbol)
            }
        }
        val function = symbol.containingDeclarationSymbol as? FirFunctionSymbol<*>
        if (function is FirAnonymousFunctionSymbol || (function != null && function.isOverride)) return Value.Unknown
        return Value.CallerSupplied
    }

    // A Java field's constant value (`static final int X = 2`).
    context(context: CheckerContext)
    private fun fieldValue(symbol: FirFieldSymbol): Value {
        if (isAndroidConstant(symbol)) return Value.ReadsConstant
        if (symbol.isVar) return Value.Unknown
        val literal = symbol.resolvedInitializer as? FirLiteralExpression ?: return Value.Unknown
        return literalValue(literal)
    }

    private fun literalValue(literal: FirLiteralExpression): Value = when (val value = literal.value) {
        is Float, is Double -> Value.NotAMode
        is Number -> Value.Known(value.toLong())
        else -> Value.NotAMode // a String, Char, Boolean, or null
    }

    // Evaluates an Int expression: literals, reads of variables whose value
    // is known, and Int/Long operators on them (`1 shl 1`, `A or B`).
    context(context: CheckerContext)
    private fun evaluate(expression: FirExpression, seen: MutableSet<FirVariableSymbol<*>>): Value = when (expression) {
        is FirLiteralExpression -> literalValue(expression)
        is FirSmartCastExpression -> evaluate(expression.originalExpression, seen)
        is FirFunctionCall -> evaluateOperator(expression, seen)
        is FirQualifiedAccessExpression -> when (val symbol = expression.calleeReference.toResolvedCallableSymbol()) {
            null -> Value.Unknown
            else -> if (isAndroidConstant(symbol)) {
                Value.ReadsConstant
            } else {
                when (val original = symbol.unwrapFakeOverrides()) {
                    is FirPropertySymbol -> propertyValue(original, seen)
                    is FirValueParameterSymbol -> parameterValue(original, seen)
                    is FirFieldSymbol -> fieldValue(original)
                    else -> Value.Unknown
                }
            }
        }
        else -> if (readsConstant(expression)) Value.ReadsConstant else Value.Unknown
    }

    context(context: CheckerContext)
    private fun evaluateOperator(call: FirFunctionCall, seen: MutableSet<FirVariableSymbol<*>>): Value {
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol
        val owner = callee?.callableId?.classId
        val receiver = call.explicitReceiver
        if (callee == null || owner !in integralClassIds || receiver == null) {
            return if (readsConstant(call)) Value.ReadsConstant else Value.Unknown
        }
        val operands = listOf(evaluate(receiver, seen)) + call.argumentList.arguments.map { evaluate(it, seen) }
        if (Value.ReadsConstant in operands) return Value.ReadsConstant
        val known = operands.map { (it as? Value.Known)?.bits ?: return Value.Unknown }
        val isLong = callee.resolvedReturnType.lowerBoundIfFlexible().classId == StandardClassIds.Long
        val result = if (isLong) longOperator(callee.name.asString(), known) else intOperator(callee.name.asString(), known)
        return result?.let { Value.Known(it) } ?: Value.Unknown
    }

    private fun intOperator(name: String, operands: List<Long>): Long? {
        val a = operands[0].toInt()
        val b = operands.getOrNull(1)?.toInt()
        val result = when (name) {
            "inv" -> a.inv()
            "unaryMinus" -> -a
            "unaryPlus", "toInt", "toShort", "toByte" -> a
            else -> when {
                b == null -> return null
                name == "or" -> a or b
                name == "and" -> a and b
                name == "xor" -> a xor b
                name == "shl" -> a shl b
                name == "shr" -> a shr b
                name == "ushr" -> a ushr b
                name == "plus" -> a + b
                name == "minus" -> a - b
                name == "times" -> a * b
                name == "div" && b != 0 -> a / b
                name == "rem" && b != 0 -> a % b
                else -> return null
            }
        }
        return result.toLong()
    }

    private fun longOperator(name: String, operands: List<Long>): Long? {
        val a = operands[0]
        val b = operands.getOrNull(1)
        return when (name) {
            "inv" -> a.inv()
            "unaryMinus" -> -a
            "unaryPlus", "toLong" -> a
            else -> when {
                b == null -> null
                name == "or" -> a or b
                name == "and" -> a and b
                name == "xor" -> a xor b
                name == "shl" -> a shl b.toInt()
                name == "shr" -> a shr b.toInt()
                name == "ushr" -> a ushr b.toInt()
                name == "plus" -> a + b
                name == "minus" -> a - b
                name == "times" -> a * b
                name == "div" && b != 0L -> a / b
                name == "rem" && b != 0L -> a % b
                else -> null
            }
        }
    }

    // The element reads Android's constant somewhere, directly or through a
    // property whose declared value does (any name, any depth).
    context(context: CheckerContext)
    private fun readsConstant(element: FirElement): Boolean =
        readsConstant(element, HashSet())

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun readsConstant(element: FirElement, seen: MutableSet<FirPropertySymbol>): Boolean {
        var found = false
        element.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirQualifiedAccessExpression) {
                    val read = element.calleeReference.toResolvedCallableSymbol()
                    if (read != null && isAndroidConstant(read)) {
                        found = true
                        return
                    }
                    val property = (read?.unwrapFakeOverrides() as? FirPropertySymbol)
                    if (property != null && seen.add(property)) {
                        val values = listOfNotNull(
                            property.resolvedInitializer,
                            property.delegate,
                            property.getterSymbol?.takeUnless { it.isDefault }?.fir?.body,
                        )
                        if (values.any { readsConstant(it, seen) }) {
                            found = true
                            return
                        }
                    }
                }
                element.acceptChildren(this)
            }
        })
        return found
    }
}
