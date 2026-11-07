package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirCallableDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirCallableDeclaration
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.types.ConeDynamicType
import org.jetbrains.kotlin.fir.types.ConeFlexibleType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.lexer.KtTokens

/**
 * Flags a public or protected function or property that has no declared type
 * while the compiler infers a platform type for it: a Java call result,
 * `File("a").name`, `System.getenv("a")` or `ArrayList` from a Java method. This is
 * detekt's `HasPlatformType`, which reads the declaration's inferred type and
 * reports a flexible one.
 *
 * Conditions, as in detekt:
 * - a function or property written in source with no declared type, which is
 *   neither local nor a member of a local class;
 * - declared public or protected (not `private` or `internal`), whatever the
 *   visibility of the enclosing class;
 * - the inferred type is flexible (`String!`, `(Mutable)List<String!>!`).
 *
 * The Go rule only reads the text of an expression-body function and reports a
 * `java.`/`javax.`/`android.` prefix, so it misses every platform type that
 * does not show the prefix and every property. Like Go, findings in test files
 * are skipped, and the finding sits on the declaration's name.
 */
internal object HasPlatformType : FirCallableDeclarationChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "HasPlatformType"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val callableDeclarationCheckers = setOf(HasPlatformType)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirCallableDeclaration) {
        if (declaration !is FirNamedFunction && declaration !is FirProperty) return
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        // A declared type has a real source. An inferred one is a fake source of the implicit-type kind.
        if (declaration.returnTypeRef.source?.kind !is KtFakeSourceElementKind) return
        val visibility = declaration.visibility
        if (visibility != Visibilities.Public && visibility != Visibilities.Protected) return
        if (context.containingDeclarations.any { it is FirClassLikeSymbol<*> && it.classId.isLocal }) return
        val type = declaration.returnTypeRef.coneType
        if (type !is ConeFlexibleType || type is ConeDynamicType) return
        if (isInTestFile()) return

        val identifier = lightChildren(source, source.lighterASTNode).firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
        val name = when (declaration) {
            is FirProperty -> declaration.name
            is FirNamedFunction -> declaration.name
            else -> return
        }.asString()
        report(identifier?.let { lightSourceOf(it, source) } ?: source, "$name has implicit platform type. Type must be declared explicitly.")
    }
}
