package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.firstModifierAnchor
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirValueParameter
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds
import org.jetbrains.kotlin.types.ConstantValueKind

/**
 * Port of the Go AllowAllHostnameVerifier rule: a
 * `javax.net.ssl.HostnameVerifier` implementation whose
 * `verify(hostname, session)` accepts every host. Reported on the function's
 * first line (its first annotation or modifier, else `fun`), the line Go
 * reports.
 *
 * The function is the `verify(String, SSLSession)` (either parameter may be
 * nullable) that a class, interface, enum class, object, companion object,
 * local class, or anonymous object declares directly as a member, where that
 * declaring class has `javax.net.ssl.HostnameVerifier` in its supertype
 * closure (so a subclass of a verifier base class, a type alias, and an import
 * alias count). Such a function can only be the override of
 * `HostnameVerifier.verify`. Its body always returns true, as Go reads it,
 * when it is the single statement `return true` (a block body, with or without
 * `;` and comments) or the expression body `= true`.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`AllowAllHostnameVerifier*.kt`):
 * - Go takes any function named `verify` with two parameters whose nearest
 *   enclosing class declaration has `HostnameVerifier` in its header text, and
 *   reports only the first one per class. So it reports a `verify(Int, Int)`
 *   or generic `verify(T, SSLSession)` overload (instead of the real override
 *   after it), a local function named
 *   `verify`, a companion's or a nested anonymous object's `verify`, and the
 *   `verify` of a class that only takes a `HostnameVerifier` constructor
 *   parameter. None of those is `HostnameVerifier.verify`, so none is
 *   reported here.
 * - Go only exempts a `HostnameVerifier` lookalike declared in the same file,
 *   so it reports a class implementing one imported from another file or
 *   package. That is not the JDK interface, so it is not reported here.
 * - Go reads an expression body as the text after its last `=`, so it reports
 *   `= hostname.isEmpty() == true`, which does not always return true.
 * - Resolution sees verifiers Go misses: an object or companion object
 *   declaration and an anonymous object outside a verifier class (Go only
 *   visits class declarations), a subclass of a verifier base class, an
 *   import alias or type alias, a qualified `javax.net.ssl.HostnameVerifier`
 *   in a file that declares its own `HostnameVerifier` (Go then skips the
 *   whole file), a second verifier nested in a verifier class (Go reports one
 *   per class), a parameter list with a trailing comma (Go counts three
 *   parameters), `return@verify true`, `= (true)`, `return (true)`, and a
 *   backticked
 *   `` `verify` ``.
 * Lambdas (`HostnameVerifier { _, _ -> true }`) are not declarations, and Go
 * does not report them either.
 */
internal object AllowAllHostnameVerifier :
    FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "AllowAllHostnameVerifier"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val simpleFunctionCheckers = setOf(AllowAllHostnameVerifier)
    }

    private val verify = Name.identifier("verify")
    private val sslPackage = FqName("javax.net.ssl")
    private val hostnameVerifier = ClassId(sslPackage, Name.identifier("HostnameVerifier"))
    private val sslSession = ClassId(sslPackage, Name.identifier("SSLSession"))

    private const val MESSAGE =
        "HostnameVerifier.verify always returns true. Validate the SSLSession hostname instead of accepting every host."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        if (declaration.name != verify) return
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (declaration.receiverParameter != null || declaration.contextParameters.isNotEmpty()) return
        val params = declaration.valueParameters
        if (params.size != 2) return
        val session = context.session
        if (!hasType(params[0], StandardClassIds.String, session) || !hasType(params[1], sslSession, session)) return
        val body = declaration.body ?: return
        // The declaring class must be the nearest container: a local function
        // named verify, even inside a verifier, is not the verifier's verify.
        val owner = context.containingDeclarations.lastOrNull { it != declaration.symbol } as? FirClassSymbol<*> ?: return
        if (!isHostnameVerifier(owner, session)) return
        if (!alwaysReturnsTrue(body)) return
        report(firstLine(source), MESSAGE)
    }

    private fun hasType(param: FirValueParameter, classId: ClassId, session: FirSession): Boolean =
        param.returnTypeRef.coneType.fullyExpandedType(session).lowerBoundIfFlexible().classId == classId

    // Supertypes come from the class's own lookup tags, which are bound to
    // local and anonymous classes, so no class id is resolved from a symbol
    // that may be local.
    private fun isHostnameVerifier(owner: FirClassSymbol<*>, session: FirSession): Boolean =
        lookupSuperTypes(owner, lookupInterfaces = true, deep = true, useSiteSession = session)
            .any { it.classId == hostnameVerifier }

    // A block body `{ return true }` and an expression body `= true` both
    // resolve to a block holding one return of the literal `true`.
    private fun alwaysReturnsTrue(body: FirBlock): Boolean {
        val returned = (body.statements.singleOrNull() as? FirReturnExpression)?.result as? FirLiteralExpression
            ?: return false
        return returned.kind == ConstantValueKind.Boolean && returned.value == true
    }

    // Go reports the function_declaration's first line: the first entry of its
    // modifier list (an annotation or a modifier keyword) when it has one,
    // else the `fun` keyword. A preceding KDoc is not part of Go's node.
    private fun firstLine(source: KtSourceElement): KtSourceElement {
        firstModifierAnchor(source)?.let { return it }
        val keyword = lightChildren(source, source.lighterASTNode)
            .firstOrNull { it.tokenType == KtTokens.FUN_KEYWORD } ?: return source
        return lightSourceOf(keyword, source)
    }
}
