package dev.jasonpearson.krit.fir.checkers.style

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.goFullMatchRegex
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.modifierKeywordAnchor
import dev.jasonpearson.krit.fir.support.simpleName
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.references.FirResolvedNamedReference
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.lexer.KtTokens

// Flags a function parameter that the function never reads, with the Go rule's scope and message
// (`Parameter 'x' is unused.`, on the parameter's name).
//
// Go matches identifiers by name inside the function and has to abstain whenever it cannot place a
// reference: a name that collides with a soft keyword, a lambda or loop variable that reuses the name,
// or a parse error in the body. This checker reads resolution instead: a parameter is used when some
// resolved reference in the file (a read, a call through a function-type parameter, a string template,
// a default value of a later parameter, a callable reference) points at its symbol. A shadowing lambda
// parameter, loop variable or local is a different symbol, so shadowing needs no special case.
//
// Covered like Go: every named function (top-level, member, extension or local) with a body. Not
// reported, like Go: `override`, `open`, `abstract`, `operator`, `actual`, `expect` and `protected`
// functions, members of an interface, framework entry points (`@Test`, `@Provides`, `@Preview`, ...),
// `@Composable` and `@Subscribe`-style handlers, an empty, `= Unit` or throw/TODO/error stub body, a
// parameter whose name matches the `allowedNames` option (a full match, like Go) or that carries
// `@Suppress("unused")`, and test files.
//
// A parameter that only a contract block mentions (`contract { returns() implies (x != null) }`) counts
// as used, because the block keeps its resolved references. When a lambda in the body declares a parameter of the same name and reads it, the
// message says so, as Go's does.
internal object UnusedParameter : FirFileChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnusedParameter"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val fileCheckers = setOf(UnusedParameter)
    }

    // The default of the option in config/default-krit.yml.
    private const val DEFAULT_ALLOWED = "ignored|expected"

    // Annotation names after which a function is called by a framework or tool (Go's entryPointAnnotationNames).
    private val entryPointAnnotations = setOf(
        "Preview", "ComposePreview", "PreviewParameter", "PreviewLightDark", "DarkPreview", "LightPreview",
        "NightPreview", "DayPreview", "Test", "ParameterizedTest", "BeforeEach", "AfterEach", "BeforeAll",
        "AfterAll", "Before", "After", "BeforeClass", "AfterClass", "ParameterizedRobolectricTestRunner.Parameters",
        "Parameters", "Provides", "Binds", "BindsInstance", "Module", "JvmStatic", "JvmName", "JvmField", "Keep",
        "UsedByReflection", "UsedByNative", "VisibleForTesting", "SerializedName", "JsonCreator", "JsonProperty",
    )

    private val handlerAnnotations = setOf("Composable", "Subscribe", "OnEvent", "EventHandler")

    private val modifierTokens = listOf(
        KtTokens.OVERRIDE_KEYWORD, KtTokens.OPEN_KEYWORD, KtTokens.ABSTRACT_KEYWORD, KtTokens.OPERATOR_KEYWORD,
        KtTokens.ACTUAL_KEYWORD, KtTokens.EXPECT_KEYWORD, KtTokens.PROTECTED_KEYWORD,
    )

    private class Candidate(val name: String, val symbol: FirBasedSymbol<*>, val anchor: KtSourceElement, val function: FirNamedFunction)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirFile) {
        if (isInTestFile()) return
        val allowed = allowedNames()
        val candidates = ArrayList<Candidate>()
        val referenced = HashSet<FirBasedSymbol<*>>()
        // Nearest enclosing class declaration is an interface: Go skips its members.
        val interfaces = ArrayList<Boolean>()
        declaration.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                when (element) {
                    is FirResolvedNamedReference -> referenced += element.resolvedSymbol
                    is FirRegularClass -> {
                        // A named object is not a class declaration in Go's tree, so it keeps the outer owner.
                        val declaresOwner = element.classKind != ClassKind.OBJECT
                        if (declaresOwner) interfaces += (element.classKind == ClassKind.INTERFACE)
                        element.acceptChildren(this)
                        if (declaresOwner) interfaces.removeLast()
                        return
                    }
                    is FirNamedFunction -> {
                        if (interfaces.lastOrNull() != true) collect(element, allowed, candidates)
                    }
                }
                element.acceptChildren(this)
            }
        })
        for (candidate in candidates) {
            if (candidate.symbol in referenced) continue
            var message = "Parameter '${candidate.name}' is unused."
            if (shadowedByLambdaParameter(candidate, referenced)) {
                message += " A lambda parameter of the same name shadows it; its uses refer to the lambda parameter."
            }
            report(candidate.anchor, message)
        }
    }

    private fun allowedNames(): Regex {
        val configured = config()["allowedNames"] as? String
        return configured?.let(::goFullMatchRegex) ?: requireNotNull(goFullMatchRegex(DEFAULT_ALLOWED))
    }

    private fun collect(function: FirNamedFunction, allowed: Regex, out: MutableList<Candidate>) {
        val source = function.source ?: return
        if (source.kind !is KtRealSourceElementKind || source.lighterASTNode.tokenType != KtNodeTypes.FUN) return
        if (function.body == null || function.valueParameters.isEmpty()) return
        if (modifierTokens.any { modifierKeywordAnchor(source, it) != null }) return
        if (function.annotations.any { isExempt(it.simpleName()) }) return
        if (isStubBody(source)) return
        for (parameter in function.valueParameters) {
            val parameterSource = parameter.source ?: continue
            if (parameterSource.kind !is KtRealSourceElementKind) continue
            val identifier = lightChildren(parameterSource, parameterSource.lighterASTNode)
                .firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: continue
            val name = lightText(parameterSource, identifier).trim('`')
            if (name.isEmpty() || allowed.matches(name) || suppressesUnused(parameterSource)) continue
            out += Candidate(name, parameter.symbol, lightSourceOf(identifier, parameterSource), function)
        }
    }

    private fun isExempt(annotation: String?): Boolean =
        annotation != null && (annotation in entryPointAnnotations || annotation in handlerAnnotations || "Preview" in annotation)

    private fun suppressesUnused(parameter: KtSourceElement): Boolean {
        val text = lightText(parameter, parameter.lighterASTNode)
        return "@Suppress" in text && ("\"unused\"" in text || "\"UNUSED_PARAMETER\"" in text)
    }

    // Go skips an empty body, `= Unit`, a short throwing block, and a body that is only TODO()/error()/throw.
    // An expression body is judged on its expression, so `=` followed by a line break counts like `= TODO()`.
    private fun isStubBody(source: KtSourceElement): Boolean {
        val node = source.lighterASTNode
        val children = lightChildren(source, node)
        val block = children.firstOrNull { it.tokenType == KtNodeTypes.BLOCK }
        if (block == null) {
            val equals = children.firstOrNull { it.tokenType == KtTokens.EQ } ?: return true
            val expression = lightText(source, node).substring(equals.endOffset - node.startOffset).trim()
            return expression == "Unit" || expression.startsWith("throw ") ||
                expression.startsWith("TODO(") || expression.startsWith("error(")
        }
        val text = lightText(source, block).trim()
        if (text == "{}" || text == "{ }") return true
        if ("throw " in text && text.startsWith("{") && text.count { it == '\n' } <= 3) return true
        return blockIsStub(source, block)
    }

    private fun blockIsStub(source: KtSourceElement, block: LighterASTNode): Boolean {
        val statements = lightChildren(source, block).filter {
            it.tokenType != KtTokens.LBRACE && it.tokenType != KtTokens.RBRACE &&
                it.tokenType !in KtTokens.WHITESPACES && it.tokenType !in KtTokens.COMMENTS
        }
        if (statements.isEmpty()) return true
        if (statements.size != 1) return false
        val text = lightText(source, statements.single()).trim()
        return text.startsWith("TODO(") || text.startsWith("error(") || text.startsWith("throw ")
    }

    private fun shadowedByLambdaParameter(candidate: Candidate, referenced: Set<FirBasedSymbol<*>>): Boolean {
        var found = false
        candidate.function.body?.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirAnonymousFunction) {
                    found = element.valueParameters.any {
                        it.source?.kind is KtRealSourceElementKind && it.name.asString() == candidate.name && it.symbol in referenced
                    }
                }
                element.acceptChildren(this)
            }
        })
        return found
    }
}
