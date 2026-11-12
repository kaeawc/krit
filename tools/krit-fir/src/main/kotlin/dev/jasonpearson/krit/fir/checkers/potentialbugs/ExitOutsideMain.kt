package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.hasAnnotation
import org.jetbrains.kotlin.fir.declarations.utils.isOverride
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.arrayElementType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags a call to `kotlin.system.exitProcess`, `java.lang.System.exit`,
// `java.lang.Runtime.exit` or `java.lang.Runtime.halt` whose nearest enclosing
// named function is not detekt's main function.
//
// Mirrors detekt 1.23.8 and the Go ExitOutsideMain rule's Kotlin path:
// - the "inside main" test walks to the nearest enclosing named function and
//   exempts the call only when that function is a main function in detekt's
//   sense: named `main`, public and not an override, taking no parameter,
//   `Array<String>` or `vararg String`, and declared at the top level or as a
//   `@JvmStatic` member of an object. Lambdas, anonymous functions, classes,
//   initializers, accessors and constructors are passed through, so
//   `thread { exitProcess(1) }` inside `main` is exempt while a local or
//   member function declared inside `main` is not;
// - a call with no enclosing named function (a property initializer, an
//   `init` block, a getter) is reported;
// - the finding is reported on the call, so on the line where the call
//   expression (including its `System.` / package qualifier) starts.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: the call must resolve to one of the functions above. Go
//   matches any call named `exitProcess` (a local function, a member, a
//   function-typed value) and any `exit` whose receiver is spelled `System`
//   or `java.lang.System`, or `Runtime.getRuntime()` for `exit` and `halt`
//   (a local `object System`).
// - Recall: an import alias of either function, a static import of
//   System.exit, an import alias of java.lang.System, or a Runtime held in a
//   variable still calls the real function; Go matches only the literal
//   spellings and misses them.
internal object ExitOutsideMain : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ExitOutsideMain"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ExitOutsideMain)
    }

    private const val MESSAGE = "Do not call exitProcess() or System.exit() outside of the main function."

    private val exitProcess = CallableId(FqName("kotlin.system"), Name.identifier("exitProcess"))
    private val systemExit = CallableId(classIdOf("java.lang", "System"), Name.identifier("exit"))
    private val runtimeClass = classIdOf("java.lang", "Runtime")
    private val runtimeExit = CallableId(runtimeClass, Name.identifier("exit"))
    private val runtimeHalt = CallableId(runtimeClass, Name.identifier("halt"))
    private val exits = setOf(exitProcess, systemExit, runtimeExit, runtimeHalt)
    private val main = Name.identifier("main")
    private val jvmStatic = classIdOf("kotlin.jvm", "JvmStatic")
    private val stringClass = classIdOf("kotlin", "String")
    private val arrayClass = classIdOf("kotlin", "Array")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val callableId = callee.callableId ?: return
        if (callableId !in exits) return

        val declarations = context.containingDeclarations
        val enclosing = declarations.indexOfLast { it is FirNamedFunctionSymbol }
        if (enclosing >= 0 && isMainFunction(declarations[enclosing] as FirNamedFunctionSymbol, declarations.getOrNull(enclosing - 1), context.session)) return

        report(expression.source, MESSAGE)
    }

    // detekt's isMainFunction: a public, non-override `main` with the standard parameters, declared at the top level
    // or as a @JvmStatic member of an object.
    private fun isMainFunction(function: FirNamedFunctionSymbol, owner: FirBasedSymbol<*>?, session: FirSession): Boolean {
        if (function.name != main || function.isOverride || function.resolvedStatus.visibility != Visibilities.Public) return false
        if (function.receiverParameterSymbol != null || !hasMainParameter(function, session)) return false
        return when (owner) {
            is FirFileSymbol -> true
            is FirRegularClassSymbol -> owner.classKind == ClassKind.OBJECT && function.hasAnnotation(jvmStatic, session)
            else -> false
        }
    }

    // No parameter, `Array<String>`, or a single `vararg String`.
    private fun hasMainParameter(function: FirNamedFunctionSymbol, session: FirSession): Boolean {
        val parameter = function.valueParameterSymbols.let { if (it.isEmpty()) return true else it.singleOrNull() } ?: return false
        val type = parameter.resolvedReturnType.fullyExpandedType(session)
        if (parameter.isVararg) return type.arrayElementType()?.fullyExpandedType(session)?.classId == stringClass
        val element = type.typeArguments.singleOrNull() as? ConeKotlinType ?: return false
        return type.classId == arrayClass && element.fullyExpandedType(session).classId == stringClass
    }
}
