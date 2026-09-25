package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags a `java.security.SecureRandom(...)` constructor call that passes any
// argument: the JDK's `SecureRandom(byte[] seed)` constructor. A supplied seed
// defeats the platform's self-seeding; the no-argument constructor should be
// used instead.
//
// Like the Go rule:
// - any seed argument counts, not only a literal one (a parameter or
//   `generateSeed()` result is reported too);
// - only constructor calls count. A superclass delegation
//   (`class S(seed: ByteArray) : SecureRandom(seed)`, `object : SecureRandom(seed) {}`)
//   and a constructor reference (`::SecureRandom`) are not calls of the
//   constructor at that site, and Go does not report them either;
// - the finding sits on the first line of the call expression, including the
//   package qualifier of a fully qualified call.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall: the constructor is identified by resolution, so an import alias
//   (`import java.security.SecureRandom as SR`), a typealias, a
//   `java.security.*` star import, a backticked name, and a file that also
//   imports `kotlin.random.Random` are reported. Go needs the literal spelling
//   `SecureRandom` plus an explicit `java.security.SecureRandom` import (and no
//   `kotlin.random.Random` import), or the literal `java.security.SecureRandom`
//   qualifier.
// - Precision: a local or nested class, or a function-typed parameter, named
//   SecureRandom that shadows the import is not java.security.SecureRandom and
//   is not reported. Go reports it from the import alone.
// - Precision: a call whose argument list holds only a comment
//   (`SecureRandom(/* no seed */)`) passes no seed and is not reported. Go counts
//   the comment node as an argument.
internal object TrulyRandom : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "TrulyRandom"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(TrulyRandom)
    }

    private const val MESSAGE =
        "SecureRandom with a hardcoded seed is not secure. Use the default constructor for cryptographic randomness."

    private val secureRandomClassId = ClassId(FqName("java.security"), Name.identifier("SecureRandom"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (expression.argumentList.arguments.isEmpty()) return
        val constructor = expression.calleeReference.toResolvedCallableSymbol() as? FirConstructorSymbol ?: return
        // The return type names the constructed class; for a typealias
        // constructor it is expanded to the aliased class.
        val constructed = constructor.resolvedReturnType.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType
            ?: return
        if (constructed.lookupTag.classId != secureRandomClassId) return
        report(expression.source, MESSAGE)
    }
}
