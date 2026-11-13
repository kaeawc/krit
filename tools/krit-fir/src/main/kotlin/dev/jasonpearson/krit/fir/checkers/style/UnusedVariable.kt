package dev.jasonpearson.krit.fir.checkers.style

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.references.FirResolvedNamedReference
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens

// Flags a local variable that nothing in its file refers to, with the Go rule's scope and message
// (`Local variable 'x' is never used.`, on the variable's name).
//
// Go matches identifiers by name inside the declaring block, skips a reference it cannot place, and
// abstains when the scope holds a parse error. This checker reads resolution instead: a variable is
// used when some resolved reference in the file (a read, an assignment, `++`, `+=`, a string template,
// a callable reference or a named argument) points at its symbol. A same-named variable in another
// scope or a member of the same name is a different symbol.
//
// Covered like Go: a `val` or `var` statement with an initializer, and every entry of a destructuring
// declaration, in a function, a lambda, an initializer block, a constructor or an accessor, and the
// entries of a `for ((a, b) in ...)` header. A name matching the `allowedNames` option (a full match, like Go)
// and test files are skipped.
//
// Not reported, like Go: a loop variable (`for (x in xs)`), a lambda parameter, a `when` subject
// variable, destructured lambda parameters, a variable declared without an initializer or delegate, and
// the synthetic locals the compiler adds for loops, destructuring and `?.`.
//
// Deliberate difference from Go: a delegated local (`val x by lazy { ... }`) is reported when unused.
// Go reads it as "no initializer" unless the delegate text happens to contain `=`.
internal object UnusedVariable : FirFileChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnusedVariable"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val fileCheckers = setOf(UnusedVariable)
    }

    // Go's default for the option (`allowedNames: 'ignored|_'` in config/default-krit.yml).
    private const val DEFAULT_ALLOWED = "ignored|_"

    private class Candidate(val name: String, val symbol: FirBasedSymbol<*>, val anchor: KtSourceElement)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirFile) {
        if (isInTestFile()) return
        val allowed = allowedNames()
        val candidates = ArrayList<Candidate>()
        val referenced = HashSet<FirBasedSymbol<*>>()
        // The compiler passes a delegated local's own property reference to its getValue/setValue calls.
        val delegating = HashSet<FirBasedSymbol<*>>()
        declaration.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                when (element) {
                    is FirResolvedNamedReference -> if (element.resolvedSymbol !in delegating) referenced += element.resolvedSymbol
                    is FirProperty -> {
                        candidate(element, allowed)?.let { candidates += it }
                        if (element.delegate != null && delegating.add(element.symbol)) {
                            element.acceptChildren(this)
                            delegating.remove(element.symbol)
                            return
                        }
                    }
                }
                element.acceptChildren(this)
            }
        })
        for (candidate in candidates) {
            if (candidate.symbol !in referenced) {
                report(candidate.anchor, "Local variable '${candidate.name}' is never used.")
            }
        }
    }

    private fun allowedNames(): Regex {
        val configured = config()["allowedNames"] as? String
        return configured?.let { runCatching { Regex(anchored(it)) }.getOrNull() } ?: Regex(anchored(DEFAULT_ALLOWED))
    }

    // Go compiles a rule's regex option as a full match: a pattern without `^`/`$` gets them, and a
    // top-level alternation is grouped first (`a|b` is `^(?:a|b)$`).
    private fun anchored(pattern: String): String {
        val start = pattern.startsWith("^")
        val end = pattern.endsWith("$") && pattern.dropLast(1).takeLastWhile { it == '\\' }.length % 2 == 0
        if (start && end) return pattern
        if ('|' !in pattern) return (if (start) "" else "^") + pattern + (if (end) "" else "$")
        var core = pattern.removePrefix("^")
        if (end) core = core.removeSuffix("$")
        return "^(?:$core)$"
    }

    private fun candidate(property: FirProperty, allowed: Regex): Candidate? {
        if (!property.isLocal) return null
        val source = property.source ?: return null
        if (source.kind !is KtRealSourceElementKind) return null
        // A declaration without an initializer or delegate is assigned later; Go skips it.
        if (property.initializer == null && property.delegate == null) return null
        val node = source.lighterASTNode
        if (!isReportedDeclaration(source, node)) return null
        val identifier = lightChildren(source, node).firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return null
        // The written name: an underscore entry has a synthetic property name.
        val name = lightText(source, identifier).trim('`')
        if (allowed.matches(name)) return null
        return Candidate(name, property.symbol, lightSourceOf(identifier, source))
    }

    // A statement-position `val`/`var` (its parent is a block), or an entry of a destructuring declaration
    // that is itself such a statement or the header of a `for`. Excludes a `when` subject variable, a
    // lambda's destructured parameter, and the compiler's synthetic locals (not real source).
    private fun isReportedDeclaration(source: KtSourceElement, node: LighterASTNode): Boolean {
        val parent = source.treeStructure.getParent(node) ?: return false
        return when (node.tokenType) {
            KtNodeTypes.PROPERTY -> parent.tokenType == KtNodeTypes.BLOCK
            KtNodeTypes.DESTRUCTURING_DECLARATION_ENTRY -> {
                if (parent.tokenType != KtNodeTypes.DESTRUCTURING_DECLARATION) return false
                val owner = source.treeStructure.getParent(parent) ?: return false
                when (owner.tokenType) {
                    KtNodeTypes.BLOCK -> true
                    KtNodeTypes.VALUE_PARAMETER -> source.treeStructure.getParent(owner)?.tokenType == KtNodeTypes.FOR
                    else -> false
                }
            }
            else -> false
        }
    }
}
