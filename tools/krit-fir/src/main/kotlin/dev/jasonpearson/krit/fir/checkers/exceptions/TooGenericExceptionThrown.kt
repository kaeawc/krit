package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtPsiSourceElement
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.toRegularClassSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.psi.KtElement

/**
 * Port of the Go TooGenericExceptionThrown rule: a `throw` of a newly
 * constructed `Exception`, `RuntimeException`, `Error`, or `Throwable`
 * (`exceptionNames`), reported on the `throw` line with Go's message.
 *
 * The thrown value counts when it, or a value it can evaluate to (an
 * `if`/`when` branch, either side of an elvis, a `try` or `catch` block, a cast
 * or `!!` operand), is a constructor call of one of the listed classes, through
 * a type alias or an import alias too. `kotlin.Exception`, `kotlin.Error` and
 * `kotlin.RuntimeException` are aliases of the `java.lang` classes, and
 * `kotlin.Throwable` is the Kotlin class. A call of a function named like the
 * class it returns (`fun Error(code: Int): Error`) counts too: Go reads the
 * call's name.
 *
 * The four default names always mean those classes (Go's FQN table). Any other
 * configured name counts for every library class of that name, however it is
 * written or imported (`java.io.IOException(..)`, a star import, or a
 * `kotlin.*` alias of a `java.util` class such as `NoSuchElementException`):
 * Go's resolver cannot resolve those and reports them. A class declared in a
 * Kotlin source of the compilation is a project class, which Go finds in its
 * class index and skips, so it is never generic. Go's index holds no Java
 * declarations, so a Java class counts even when it is compiled from source.
 *
 * Exemptions mirrored from Go: test files and `.gradle.kts` scripts, and a
 * constructor that passes the parameter of the nearest `catch` enclosing it
 * directly as an argument (`throw RuntimeException("context", e)`), which
 * wraps the caught exception instead of hiding it. For a value produced by the
 * catch block of a thrown `try` expression, that catch is the nearest one.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`TooGenericExceptionThrown*.kt`) or in `TooGenericExceptionThrownFilesTest`:
 * - Go's dispatch node is any jump expression, so it also reports
 *   `return Exception(...)`, which throws nothing, and reports
 *   `return x ?: throw Exception(...)` a second time on the `return` line.
 *   Each throw is reported once, on the `throw` line, here.
 * - Go reads only the first call inside the `throw` (`throw if (c)
 *   IllegalStateException() else Exception()` names IllegalStateException),
 *   the name as written (an import alias or type alias hides the class), and
 *   resolves an explicit `import java.lang.Exception` to `kotlin.Exception`,
 *   which is not in its table; it also skips every throw of a name the file
 *   declares a class for anywhere, even a nested class the throw does not
 *   resolve to. Each of those throws a generic exception, so each is reported
 *   here.
 * - Go resolves a name without an explicit import or a same-file class to
 *   `java.lang`, so it reports throws that construct a project class or a
 *   type alias of another class: a same-package or star-imported class named
 *   `Exception`, a nested class of a supertype, a same-package or same-file
 *   `typealias Error = ...`. None of them throws a generic exception.
 * - With a configured extra name, Go skips a library class that is imported
 *   explicitly (it resolves the import to an FQN outside its table); that
 *   class is still reported here.
 * - Go matches the caught exception by name, so a lambda parameter shadowing
 *   the catch parameter exempts the throw there; here the argument must be
 *   the catch parameter itself.
 */
internal object TooGenericExceptionThrown :
    FirExpressionChecker<FirThrowExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "TooGenericExceptionThrown"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val throwExpressionCheckers = setOf(TooGenericExceptionThrown)
    }

    private val defaultNames = listOf("Exception", "Throwable", "Error", "RuntimeException")

    private val javaLang = FqName("java.lang")
    private val kotlinPackage = FqName("kotlin")

    private val genericClassIds = setOf(
        ClassId(javaLang, Name.identifier("Exception")),
        ClassId(javaLang, Name.identifier("RuntimeException")),
        ClassId(javaLang, Name.identifier("Error")),
        ClassId(javaLang, Name.identifier("Throwable")),
        ClassId(kotlinPackage, Name.identifier("Throwable")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirThrowExpression) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (isInTestFile()) return
        if (containingScanPath()?.endsWith(".gradle.kts") == true) return

        val names = exceptionNames()
        val caught = (context.containingElements.lastOrNull { it is FirCatch } as? FirCatch)?.parameter?.symbol

        val candidates = mutableListOf<Candidate>()
        collectResults(expression.exception, caught, candidates, depth = 0)
        for (candidate in candidates) {
            val call = candidate.value as? FirFunctionCall ?: continue
            val name = genericClassName(call, names) ?: continue
            val nearestCatch = candidate.caught
            if (nearestCatch != null && passesCaught(call, nearestCatch)) continue
            report(throwKeyword(source), "Too-generic exception type '$name' thrown.")
            return
        }
    }

    context(context: CheckerContext)
    private fun exceptionNames(): Set<String> {
        val configured = (config()["exceptionNames"] as? List<*>)?.mapNotNull { it as? String }.orEmpty()
        return configured.ifEmpty { defaultNames }.toSet()
    }

    // A value the thrown expression can evaluate to, with the parameter of the
    // nearest catch enclosing that value.
    private class Candidate(val value: FirExpression, val caught: FirBasedSymbol<*>?)

    // The thrown expression and every value it can evaluate to, in source
    // order. A value in the catch block of a thrown `try` has that catch as
    // its nearest one; every other value has the throw's.
    private fun collectResults(
        expression: FirExpression,
        caught: FirBasedSymbol<*>?,
        into: MutableList<Candidate>,
        depth: Int,
    ) {
        into += Candidate(expression, caught)
        if (depth > MAX_DEPTH) return
        val next = depth + 1
        when (expression) {
            is FirWhenExpression -> expression.branches.forEach { collectResults(it.result, caught, into, next) }
            is FirElvisExpression -> {
                collectResults(expression.lhs, caught, into, next)
                collectResults(expression.rhs, caught, into, next)
            }
            is FirTryExpression -> {
                collectResults(expression.tryBlock, caught, into, next)
                expression.catches.forEach { collectResults(it.block, it.parameter.symbol, into, next) }
            }
            is FirBlock -> (expression.statements.lastOrNull() as? FirExpression)?.let {
                collectResults(it, caught, into, next)
            }
            is FirTypeOperatorCall -> if (expression.operation == FirOperation.AS || expression.operation == FirOperation.SAFE_AS) {
                expression.argumentList.arguments.singleOrNull()?.let { collectResults(it, caught, into, next) }
            }
            is FirCheckNotNullCall -> expression.argumentList.arguments.singleOrNull()?.let {
                collectResults(it, caught, into, next)
            }
            is FirSmartCastExpression -> collectResults(expression.originalExpression, caught, into, next)
            else -> Unit
        }
    }

    // The simple name of the generic exception class [call] constructs, or
    // null. The class id comes from the call's type's lookup tag; the class is
    // only resolved once its id is known not to be local.
    context(context: CheckerContext)
    private fun genericClassName(call: FirFunctionCall, names: Set<String>): String? {
        val callee = call.calleeReference.toResolvedCallableSymbol() ?: return null
        val type = call.resolvedType.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType
            ?: return null
        val classId = type.lookupTag.classId
        if (classId.isLocal) return null
        val name = classId.shortClassName.asString()
        if (callee !is FirConstructorSymbol && callee.callableId?.callableName?.asString() != name) return null
        if (name !in names) return null
        if (classId in genericClassIds) return name
        // Go binds the four default names to its java.lang FQN table.
        if (name in defaultNames) return null
        // Any other configured name counts for a library or Java class of
        // that name, however it is written or imported (Go reports it
        // unresolved), and never for a class declared in Kotlin source (Go
        // finds it in its class index, which holds Kotlin declarations only).
        val symbol = type.lookupTag.toRegularClassSymbol(context.session) ?: return null
        return name.takeUnless { declaredInKotlinSource(symbol) }
    }

    // Whether [symbol] is declared in a Kotlin file of this compilation: its
    // source is a real element of the Kotlin light tree (how the compiler
    // parses Kotlin sources) or of Kotlin PSI. A library class has no source,
    // and a Java class compiled from a source root has a Java PSI element.
    private fun declaredInKotlinSource(symbol: FirRegularClassSymbol): Boolean {
        val source = symbol.source ?: return false
        if (source.kind !is KtRealSourceElementKind) return false
        return source !is KtPsiSourceElement || source.psi is KtElement
    }

    // Whether [call] passes the catch parameter itself as an argument.
    private fun passesCaught(call: FirFunctionCall, caught: FirBasedSymbol<*>): Boolean =
        call.argumentList.arguments.any { argument ->
            var value = argument
            if (value is FirWrappedArgumentExpression) value = value.expression
            if (value is FirSmartCastExpression) value = value.originalExpression
            val access = value as? FirPropertyAccessExpression ?: return@any false
            access.explicitReceiver == null && access.calleeReference.toResolvedVariableSymbol() == caught
        }

    // Go reports at the throw node's start: the `throw` keyword.
    private fun throwKeyword(source: KtSourceElement): KtSourceElement {
        val keyword = lightChildren(source, source.lighterASTNode).firstOrNull { it.tokenType == KtTokens.THROW_KEYWORD }
            ?: return source
        return lightSourceOf(keyword, source)
    }

    private const val MAX_DEPTH = 16
}
