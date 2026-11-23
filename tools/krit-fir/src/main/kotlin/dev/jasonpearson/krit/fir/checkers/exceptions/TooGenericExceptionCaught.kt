package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.expandedClassType
import dev.jasonpearson.krit.fir.support.goFullMatchRegex
import dev.jasonpearson.krit.fir.support.identifierAnchor
import dev.jasonpearson.krit.fir.support.identifierText
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.trimGoSpace
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.name.ClassId

// Flags a catch clause whose parameter type is one of the generic exceptions in `exceptionNames`.
//
// Mirrors the Go TooGenericExceptionCaught rule, a port of detekt that compares the text of the caught
// type with the configured names. This checker compares the resolved class, so a type alias or import
// alias of a generic exception is reported, and a project class that only shares the simple name is not.
//
// Same as Go: the finding sits on the parameter name; a parameter whose name matches
// a plain Go-normalized `allowedExceptionNameRegex` (full match) is skipped; unsupported configured
// syntax makes this checker abstain. Nothing about the catch body matters; and a type
// written with a qualifier (`java.lang.Exception`) or an annotation (`@Suppress(...) Exception`) is not
// matched, because detekt's contract is the simple name as written. The annotation form is how code
// suppresses the rule on a catch parameter.
//
// The default names mean their canonical classes (`java.lang.*`, `kotlin.Throwable`). A name added to
// `exceptionNames` matches a caught class of that simple name in any package, as Go's text match does.
internal object TooGenericExceptionCaught :
    FirExpressionChecker<FirTryExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "TooGenericExceptionCaught"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val tryExpressionCheckers = setOf(TooGenericExceptionCaught)
    }

    private const val MESSAGE =
        "The caught exception is too generic. " +
            "Prefer catching specific exceptions to the case that is currently handled."

    // Defaults of config/default-krit.yml.
    private const val DEFAULT_ALLOWED = "_|(ignore|expected).*"
    private val defaultNames = listOf(
        "Exception", "Throwable", "Error", "RuntimeException", "NullPointerException",
        "ArrayIndexOutOfBoundsException", "IndexOutOfBoundsException", "IllegalMonitorStateException",
    )

    // A bare identifier; a qualifier or an annotation makes the written text something else.
    private val simpleName = Regex("[\\p{L}_][\\p{L}\\p{N}_]*")

    // The classes the default names stand for. `Throwable` is `kotlin.Throwable` or its Java twin.
    private val canonical: Map<String, Set<ClassId>> = defaultNames.associateWith { name ->
        val javaClass = classIdOf("java.lang", name)
        if (name == "Throwable") setOf(javaClass, classIdOf("kotlin", name)) else setOf(javaClass)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTryExpression) {
        val names = exceptionNames()
        val allowed = allowedNames() ?: return
        expression.catches.forEach { checkCatch(it, names, allowed) }
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    private fun checkCatch(catch: FirCatch, names: Set<String>, allowed: Regex) {
        val source = catch.parameter.source ?: return
        val anchor = identifierAnchor(source) ?: return
        val name = identifierText(source) ?: return
        if (isGeneric(catch, names) && !allowed.matches(name)) report(anchor, MESSAGE)
    }

    context(context: CheckerContext)
    private fun isGeneric(catch: FirCatch, names: Set<String>): Boolean {
        val typeRef = catch.parameter.returnTypeRef
        val written = typeRef.source?.let { trimGoSpace(lightText(it, it.lighterASTNode)) } ?: return false
        if (!simpleName.matches(written)) return false
        val classId = typeRef.coneType.expandedClassType()?.lookupTag?.classId ?: return false
        val name = classId.shortClassName.asString()
        if (name !in names) return false
        val expected = canonical[name] ?: return true
        return classId in expected
    }

    private fun exceptionNames(): Set<String> {
        val configured = (config()["exceptionNames"] as? List<*>)?.mapNotNull { it as? String }.orEmpty()
        return configured.ifEmpty { defaultNames }.toSet()
    }

    private fun allowedNames(): Regex? {
        val configured = config()["allowedExceptionNameRegex"] as? String
        return if (configured != null) {
            if (isPlainPattern(configured)) goFullMatchRegex(configured) else null
        } else {
            requireNotNull(goFullMatchRegex(DEFAULT_ALLOWED))
        }
    }

    // A plain pattern means the same in Go's RE2 and Java: no escapes, flags, nested or POSIX classes, bounded or
    // stacked quantifiers, and no anchors (Go searches an already-anchored pattern while FIR full-matches it).
    private val unplainParts = listOf("\\", "(?", "[[", "[:", "&&", "{")
    private val plainClass = Regex("""\[\^?(?:][\p{L}\p{Nd}_.|()\-*+?]*|[\p{L}\p{Nd}_.|()\-*+?]+)]""")
    private val stackedQuantifier = Regex("[*+?]{2}")

    private fun isPlainPattern(pattern: String): Boolean {
        val unplain = unplainParts.any { it in pattern } || pattern.any { it.isSurrogate() }
        if (unplain || stackedQuantifier.containsMatchIn(pattern)) return false
        return plainClass.replace(pattern, "C").all { it.isLetterOrDigit() || it in "_.|()-*+?" }
    }
}
