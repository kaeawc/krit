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

    @Test
    fun expressionTypeReturnsCallResolvedTypeFqn() {
        val source = "package x\n\nfun caller() {\n    target()\n}\n"
        val table = FileOffsetTable(source)
        val expressions = mapOf(
            "4:5" to ExpressionPayload(
                type = "kotlin.String",
                callTarget = "com.acme.target",
                callTargetResolved = true,
            ),
        )
        val (parsed, call) = parseSourceAndFirstCall(source)
        parsed.use {
            val resolver = FirOracleResolver(expressions, table)
            assertEquals("kotlin.String", resolver.expressionType(call))
        }
    }

    @Test
    fun expressionTypeReturnsNullWhenPayloadHasEmptyTypeString() {
        // Pre-oracle versions left `type=""` because no Go-side
        // consumer read it. The resolver must surface that as
        // "unresolved" → null rather than as the empty string so a
        // rule's null check still works.
        val source = "package x\n\nfun caller() {\n    target()\n}\n"
        val table = FileOffsetTable(source)
        val expressions = mapOf(
            "4:5" to ExpressionPayload(
                type = "",
                callTarget = "com.acme.target",
                callTargetResolved = true,
            ),
        )
        val (parsed, call) = parseSourceAndFirstCall(source)
        parsed.use {
            val resolver = FirOracleResolver(expressions, table)
            assertNull(resolver.expressionType(call))
        }
    }

    @Test
    fun expressionTypeServesNonCallEntriesFromTheSameMap() {
        // After the qualified-access broadening, property reads and
        // variable references also land in `expressionsByKey` —
        // with `callTargetResolved=false` so they don't interfere
        // with call-resolution lookups. The resolver looks up by
        // PSI offset → line:col regardless of expression subtype.
        val source = "package x\n\nval name: String = \"hi\"\nfun caller() {\n    val s = name\n}\n"
        val table = FileOffsetTable(source)
        val parsed = KtFileParser.parse(source, pathHint = "/tmp/Source.kt")
        parsed.use {
            var ref: org.jetbrains.kotlin.psi.KtNameReferenceExpression? = null
            parsed.ktFile.accept(object : org.jetbrains.kotlin.psi.KtTreeVisitorVoid() {
                override fun visitReferenceExpression(expression: org.jetbrains.kotlin.psi.KtReferenceExpression) {
                    super.visitReferenceExpression(expression)
                    if (
                        ref == null &&
                        expression is org.jetbrains.kotlin.psi.KtNameReferenceExpression &&
                        expression.getReferencedName() == "name"
                    ) {
                        ref = expression
                    }
                }
            })
            val nameRef = ref ?: error("source has no `name` reference: $source")
            val (line, col) = table.lineColAt(nameRef.textRange.startOffset)
            val expressions = mapOf(
                "$line:$col" to ExpressionPayload(
                    type = "kotlin.String",
                    nullable = false,
                    callTarget = null,
                    callTargetResolved = false,
                ),
            )
            val resolver = FirOracleResolver(expressions, table)
            assertEquals("kotlin.String", resolver.expressionType(nameRef))
        }
    }

    @Test
    fun isLambdaSuspendLooksUpByPsiOffset() {
        // The PSI offset of a KtLambdaExpression sits on its opening
        // brace; K2 records the FirAnonymousFunction's source at the
        // same position, so `"line:col"` round-trips.
        val source = "package x\n\nfun caller() {\n    run {\n        target()\n    }\n}\n"
        val table = FileOffsetTable(source)
        val parsed = KtFileParser.parse(source, pathHint = "/tmp/Source.kt")
        parsed.use {
            val foundLambda = firstLambdaExpression(parsed.ktFile)
                ?: error("source has no lambda")
            val (line, col) = table.lineColAt(foundLambda.textRange.startOffset)

            val suspendResolver = FirOracleResolver(
                expressionsByKey = emptyMap(),
                offsets = table,
                lambdaSuspendByKey = mapOf("$line:$col" to true),
            )
            assertTrue(suspendResolver.isLambdaSuspend(foundLambda))

            val nonSuspendResolver = FirOracleResolver(
                expressionsByKey = emptyMap(),
                offsets = table,
                lambdaSuspendByKey = mapOf("$line:$col" to false),
            )
            assertFalse(nonSuspendResolver.isLambdaSuspend(foundLambda))

            // Empty map → conservative false. Matches krit-types'
            // "unresolved → false" contract so rules don't see a
            // spurious "suspend" for lambdas the oracle didn't visit.
            val noDataResolver = FirOracleResolver(
                expressionsByKey = emptyMap(),
                offsets = table,
                lambdaSuspendByKey = emptyMap(),
            )
            assertFalse(noDataResolver.isLambdaSuspend(foundLambda))
        }
    }

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

    private fun firstLambdaExpression(ktFile: KtFile): org.jetbrains.kotlin.psi.KtLambdaExpression? {
        var found: org.jetbrains.kotlin.psi.KtLambdaExpression? = null
        ktFile.accept(object : org.jetbrains.kotlin.psi.KtTreeVisitorVoid() {
            override fun visitLambdaExpression(expression: org.jetbrains.kotlin.psi.KtLambdaExpression) {
                if (found == null) found = expression
            }
        })
        return found
    }
}
