package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.goFullMatchRegex
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.references.FirResolvedNamedReference
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens

// Flags a catch clause that loses the caught exception: the body never refers to it, or every `throw`
// in the body that mentions it builds the new exception from its members only (`e.message`).
//
// Mirrors the Go SwallowedException rule, itself a port of detekt. Go finds the caught variable's uses
// by identifier text, so a lambda parameter, local or property that reuses the name counts as a use and
// hides a swallow. This checker resolves references: only an access to the catch parameter's own symbol
// is a use, and the identifier text of a throw never decides anything.
//
// Same as Go:
// - the type exemption: a catch whose written type contains an `ignoredExceptionTypes` entry
//   (case-insensitive, `*` trimmed) is skipped, as is a name matching `allowedExceptionNameRegex` (full
//   match);
// - a body name that equals an ignored type (`NumberFormatException(...)`, `X::class`) counts as a use;
// - a `throw` is a swallow when it reads the exception, directly or through a local declared in the
//   catch body, only through qualified chains (`e.message`, `e.cause?.message`, `e.toString()`), none of
//   them thrown as they stand. A bare `e`, `e?.m`, or `foo(e)` argument consumes the exception.
//
// Deliberate differences from Go: a typealias of an ignored type is exempt too, a `$e` template entry
// is a bare use, and Go's name-based lookups of shadowed names are gone.
internal object SwallowedException : FirExpressionChecker<FirTryExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "SwallowedException"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val tryExpressionCheckers = setOf(SwallowedException)
    }

    private const val MESSAGE = "The caught exception is swallowed. The original exception could be lost."

    // Defaults of config/default-krit.yml.
    private val DEFAULT_IGNORED = listOf("InterruptedException", "MalformedURLException", "NumberFormatException", "ParseException")
    private const val DEFAULT_ALLOWED = "_|(ignore|expected).*"

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTryExpression) {
        val ignored = ignoredTypes()
        val allowed = allowedNames()
        for (catch in expression.catches) {
            if (isSwallowed(catch, ignored, allowed)) report(catchKeyword(catch) ?: continue, MESSAGE)
        }
    }

    // Go reports on the `catch` keyword, not on the whole clause.
    private fun catchKeyword(catch: FirCatch): KtSourceElement? {
        val source = catch.source ?: return null
        val keyword = lightChildren(source, source.lighterASTNode).firstOrNull { it.tokenType == KtTokens.CATCH_KEYWORD }
        return keyword?.let { lightSourceOf(it, source) }
    }

    context(context: CheckerContext)
    private fun isSwallowed(catch: FirCatch, ignored: List<String>, allowed: Regex): Boolean {
        val name = writtenName(catch) ?: return false
        if (hasIgnoredType(catch, ignored)) return false
        val parameter = catch.parameter.symbol
        val body = BodyScan(parameter, ignored.toSet()).also { catch.block.accept(it) }
        val reported = !body.used || body.throws.any { swallowsViaMembers(it, body) }
        return reported && !allowed.matches(name)
    }

    // The name as written: an underscore parameter has a synthetic name.
    private fun writtenName(catch: FirCatch): String? {
        val source = catch.parameter.source ?: return null
        val identifier = lightChildren(source, source.lighterASTNode).firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
        return identifier?.let { lightText(source, it).trim('`') }
    }

    context(context: CheckerContext)
    private fun hasIgnoredType(catch: FirCatch, ignored: List<String>): Boolean {
        val typeRef = catch.parameter.returnTypeRef
        val written = typeRef.source?.let { lightText(it, it.lighterASTNode) }.orEmpty().lowercase()
        val expanded = typeRef.coneType.fullyExpandedType().classId?.asFqNameString().orEmpty().lowercase()
        return ignored.any { entry ->
            val needle = entry.removePrefix("*").removeSuffix("*").lowercase()
            written.contains(needle) || expanded.contains(needle)
        }
    }

    private fun swallowsViaMembers(thrown: FirThrowExpression, body: BodyScan): Boolean {
        val direct = RefScan(body.parameter).also { thrown.accept(it) }
        val shown = direct.refs.toMutableList()
        for (symbol in direct.reads) {
            val initializer = body.locals[symbol]?.initializer ?: continue
            shown += RefScan(body.parameter).also { initializer.accept(it) }.refs
        }
        return shown.isNotEmpty() && shown.all { it }
    }

    private fun ignoredTypes(): List<String> =
        (config()["ignoredExceptionTypes"] as? List<*>)?.mapNotNull { it as? String } ?: DEFAULT_IGNORED

    private fun allowedNames(): Regex {
        val configured = config()["allowedExceptionNameRegex"] as? String
        return configured?.let(::goFullMatchRegex) ?: requireNotNull(goFullMatchRegex(DEFAULT_ALLOWED))
    }

    // What the catch body does: whether it uses the parameter, its throws, and the locals it declares.
    private class BodyScan(val parameter: FirBasedSymbol<*>, private val ignoredNames: Set<String>) : FirVisitorVoid() {
        var used = false
        val throws = ArrayList<FirThrowExpression>()
        val locals = HashMap<FirBasedSymbol<*>, FirProperty>()

        override fun visitElement(element: FirElement) {
            when (element) {
                is FirResolvedNamedReference -> {
                    if (element.resolvedSymbol == parameter || element.name.asString() in ignoredNames) used = true
                }
                is FirResolvedQualifier -> if (element.relativeClassFqName?.shortName()?.asString() in ignoredNames) used = true
                is FirThrowExpression -> throws += element
                is FirProperty -> if (element.isLocal) locals[element.symbol] = element
            }
            element.acceptChildren(this)
        }
    }

    // Reads inside one expression: for each access to the parameter, whether it is the receiver of a
    // qualified chain that is not thrown as it stands, and the locals the expression reads.
    private class RefScan(private val parameter: FirBasedSymbol<*>) : FirVisitorVoid() {
        val refs = ArrayList<Boolean>()
        val reads = LinkedHashSet<FirBasedSymbol<*>>()
        private val stack = ArrayList<FirElement>()

        override fun visitElement(element: FirElement) {
            stack += element
            if (element is FirPropertyAccessExpression) {
                val symbol = element.calleeReference.toResolvedVariableSymbol()
                if (symbol == parameter) refs += isMemberOnly() else if (symbol != null) reads += symbol
            }
            element.acceptChildren(this)
            stack.removeLast()
        }

        // Climbs `e.a.b()` from the access to the outermost qualified expression it receives. A safe call
        // directly on the access (`e?.m`) is a bare use, but `e.a?.b` keeps climbing.
        private fun isMemberOnly(): Boolean {
            var index = stack.lastIndex
            var climbed = false
            while (true) {
                val parentIndex = parentIndex(index)
                if (parentIndex < 0) break
                val parent = stack[parentIndex]
                val receiver = when {
                    parent is FirSafeCallExpression -> parent.receiver.takeIf { climbed }
                    parent is FirQualifiedAccessExpression -> parent.explicitReceiver
                    else -> null
                } ?: break
                if (unwrap(receiver) !== unwrap(stack[index] as FirExpression)) break
                index = parentIndex
                climbed = true
            }
            val parentIndex = parentIndex(index)
            return climbed && (parentIndex < 0 || stack[parentIndex] !is FirThrowExpression)
        }

        private fun parentIndex(index: Int): Int {
            var i = index - 1
            while (i >= 0 && (stack[i] is FirSmartCastExpression || stack[i] is FirCheckedSafeCallSubject)) i--
            return i
        }

        private fun unwrap(expression: FirExpression): FirExpression =
            unwrapSmartCasts(expression, throughSafeCallSubject = false)
    }
}
