package dev.jasonpearson.krit.fir.plugins

import dev.jasonpearson.krit.fir.oracle.ExpressionPayload
import dev.jasonpearson.krit.fir.oracle.FileOffsetTable
import org.jetbrains.kotlin.psi.KtCallExpression
import org.jetbrains.kotlin.psi.KtFile
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class FirOracleResolverTest {

    @Test
    fun resolvedCallFqNameRoundTripsThroughLineColLookup() {
        // Source has the call at (line 4, col 5):
        // "package x\n\nfun caller() {\n    target()\n}\n"
        // line 4 starts at offset 27 ('    target...'); col 5 = 't'.
        val source = "package x\n\nfun caller() {\n    target()\n}\n"
        val table = FileOffsetTable(source)
        val expressions = mapOf(
            "4:5" to ExpressionPayload(
                type = "",
                callTarget = "com.acme.target",
                callTargetResolved = true,
                callTargetSuspend = false,
            ),
        )

        val (parsed, call) = parseSourceAndFirstCall(source)
        parsed.use {
            val resolver = FirOracleResolver(expressions, table)
            assertEquals("com.acme.target", resolver.resolvedCallFqName(call))
            assertFalse(resolver.isSuspendCall(call))
        }
    }

    @Test
    fun unresolvedCallReturnsNullEvenWithLexicalFallbackEntry() {
        // krit-types' AnalysisApiResolver returns null when the symbol
        // graph couldn't recover an FQN. We do the same: only resolved
        // FQNs surface; lexical-fallback entries don't.
        val source = "package x\n\nfun caller() {\n    mystery()\n}\n"
        val table = FileOffsetTable(source)
        val expressions = mapOf(
            "4:5" to ExpressionPayload(
                type = "",
                callTarget = "mystery",
                callTargetResolved = false,
                callTargetSuspend = false,
            ),
        )

        val (parsed, call) = parseSourceAndFirstCall(source)
        parsed.use {
            val resolver = FirOracleResolver(expressions, table)
            assertNull(resolver.resolvedCallFqName(call))
        }
    }

    @Test
    fun suspendFlagSurfacesViaIsSuspendCall() {
        val source = "package x\n\nsuspend fun caller() {\n    delay()\n}\n"
        val table = FileOffsetTable(source)
        val expressions = mapOf(
            "4:5" to ExpressionPayload(
                type = "",
                callTarget = "kotlinx.coroutines.delay",
                callTargetResolved = true,
                callTargetSuspend = true,
            ),
        )

        val (parsed, call) = parseSourceAndFirstCall(source)
        parsed.use {
            val resolver = FirOracleResolver(expressions, table)
            assertTrue(resolver.isSuspendCall(call))
            assertEquals("kotlinx.coroutines.delay", resolver.resolvedCallFqName(call))
        }
    }

    @Test
    fun unknownCallSiteReturnsNull() {
        // Empty expression map → resolver has nothing to surface and
        // returns null/false. Mirrors krit-types' behavior when the
        // call wasn't resolvable in this file's analysis.
        val source = "package x\n\nfun caller() {\n    target()\n}\n"
        val table = FileOffsetTable(source)
        val (parsed, call) = parseSourceAndFirstCall(source)
        parsed.use {
            val resolver = FirOracleResolver(expressionsByKey = emptyMap(), offsets = table)
            assertNull(resolver.resolvedCallFqName(call))
            assertFalse(resolver.isSuspendCall(call))
        }
    }

    // PR 3.3 deliberately ships isLambdaSuspend / expressionType as
    // conservative null/false stubs because the oracle pass doesn't
    // capture lambda-functional or expression-type data yet. Those
    // methods have no behavior to test until the data lands — they
    // pass the `Resolver` contract trivially.

    // ── Test plumbing ────────────────────────────────────────────────

    private fun parseSourceAndFirstCall(
        source: String,
    ): Pair<KtFileParser.ParsedKtFile, KtCallExpression> {
        val parsed = KtFileParser.parse(source, pathHint = "/tmp/Source.kt")
        val call = firstCallExpression(parsed.ktFile)
            ?: error("source has no KtCallExpression: $source")
        return parsed to call
    }

    private fun firstCallExpression(ktFile: KtFile): KtCallExpression? {
        var found: KtCallExpression? = null
        ktFile.accept(object : org.jetbrains.kotlin.psi.KtTreeVisitorVoid() {
            override fun visitCallExpression(expression: KtCallExpression) {
                if (found == null) found = expression
            }
        })
        return found
    }
}
