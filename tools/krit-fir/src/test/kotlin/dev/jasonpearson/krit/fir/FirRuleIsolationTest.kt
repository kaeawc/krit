package dev.jasonpearson.krit.fir

import com.intellij.openapi.progress.ProcessCanceledException
import dev.jasonpearson.krit.fir.checkers.protocol.ProtocolProbe
import dev.jasonpearson.krit.fir.checkers.protocol.ThrowingProbe
import dev.jasonpearson.krit.fir.checkers.protocol.TypeProtocolProbe
import dev.jasonpearson.krit.fir.oracle.OracleExpressionChecker
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import java.util.concurrent.CancellationException
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertSame
import kotlin.test.assertTrue

class FirRuleIsolationTest {
    @TempDir lateinit var tmp: Path

    private val stdlib = File(Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath

    private fun source(name: String, text: String): File = tmp.resolve(name).toFile().apply { writeText(text) }

    @Test fun checkerExceptionIsIsolatedToItsRuleAndFile() {
        val api = source(
            "Api.kt",
            """
            fun protocolProbe() {}
            fun throwingProbeReport() {}
            fun throwingProbeCrash() {}
            """.trimIndent(),
        )
        // One file per checker family that throws, plus one the probe never throws on.
        val throwsInCall = source("ThrowsInCall.kt", "fun a() { throwingProbeReport(); throwingProbeCrash(); protocolProbe() }\n")
        val throwsInClass = source("ThrowsInClass.kt", "class ThrowingProbeCrashClass { fun b() { protocolProbe() } }\n")
        val throwsInType = source("ThrowsInType.kt", "class ThrowingProbeCrashType\nval c: ThrowingProbeCrashType? = null\nfun d() { protocolProbe() }\n")
        val clean = source("Clean.kt", "fun e() { throwingProbeReport(); protocolProbe() }\n")
        ThrowingProbe.reset()

        val requested = listOf(api, throwsInCall, throwsInClass, throwsInType, clean).map { FileRef(it.absolutePath) }
        val result = AnalysisSession(listOf(tmp.toString()), listOf(stdlib))
            .check(7, requested, setOf("ThrowingProbe", "ProtocolProbe"))

        // The compile survived: nothing crashed, nothing is gated file-wide.
        assertEquals(emptyMap(), result.crashed, result.toString())
        assertEquals(emptyMap(), result.errorFiles, result.toString())
        assertTrue(ThrowingProbe.expressionThrows.get() > 0, "expression checker never threw")
        assertTrue(ThrowingProbe.declarationThrows.get() > 0, "declaration checker never threw")
        assertTrue(ThrowingProbe.typeThrows.get() > 0, "type checker never threw")
        assertEquals(requested.size, result.succeeded)

        // Only the throwing rule, only on the files it threw on.
        assertEquals(setOf("ThrowingProbe"), result.ruleErrors.keys, result.toString())
        val errors = result.ruleErrors.getValue("ThrowingProbe")
        assertEquals(
            setOf(throwsInCall.absolutePath, throwsInClass.absolutePath, throwsInType.absolutePath),
            errors.keys,
        )
        val callError = errors.getValue(throwsInCall.absolutePath)
        assertTrue("IllegalStateException" in callError && ThrowingProbe.MESSAGE in callError, callError)
        assertFalse("second line" in callError, callError)
        assertTrue("UnsupportedOperationException" in errors.getValue(throwsInClass.absolutePath))
        assertTrue("StackOverflowError" in errors.getValue(throwsInType.absolutePath))

        // Every other rule, and the throwing rule on other files, still reports.
        for (file in listOf(throwsInCall, throwsInClass, throwsInType, clean)) {
            assertTrue(
                result.findings.any { it.rule == "ProtocolProbe" && it.path == file.absolutePath },
                "ProtocolProbe finding missing in ${file.name}: $result",
            )
        }
        assertTrue(result.findings.any { it.rule == "ThrowingProbe" && it.path == clean.absolutePath }, result.toString())

        val response = buildCheckResponse(result)
        assertTrue(
            "\"ruleErrors\":{\"ThrowingProbe\":{${jsonStr(throwsInCall.absolutePath)}:" in response,
            response,
        )
        assertTrue("""probe \"boom\" \\ on purpose""" in response, response)
    }

    @Test fun ruleCheckersAreWrappedOnlyWhenACheckCompileRecordsErrors() {
        val base = object : ExpressionCheckers() {
            override val functionCallCheckers = setOf(OracleExpressionChecker)
        }
        val recorder = FirRuleErrorRecorder()
        val isolated = mergeFirRules(listOf(ProtocolProbe, TypeProtocolProbe), baseExpressions = listOf(base), recorder = recorder)
        val calls = isolated.expression.functionCallCheckers
        // krit-fir's own base checkers stay as registered.
        assertTrue(OracleExpressionChecker in calls)
        val wrapped = calls.filterIsInstance<IsolatedExpressionChecker<*>>().single()
        assertEquals("ProtocolProbe", wrapped.ruleId)
        assertSame(ProtocolProbe, wrapped.delegate)
        assertSame(ProtocolProbe, unwrapChecker(wrapped))
        val typeWrapped = isolated.type.resolvedTypeRefCheckers.filterIsInstance<IsolatedTypeChecker<*>>().single()
        assertSame(TypeProtocolProbe, typeWrapped.delegate)
        assertEquals(wrapped.mppKind, ProtocolProbe.mppKind)

        // Outside a check compile (oracle, direct compiler runs) nothing is wrapped.
        val plain = mergeFirRules(listOf(ProtocolProbe), baseExpressions = listOf(base))
        assertTrue(ProtocolProbe in plain.expression.functionCallCheckers)
        assertTrue(plain.expression.functionCallCheckers.none { it is IsolatedExpressionChecker<*> })
    }

    @Test fun cancellationAndJvmFailuresAreNotSwallowed() {
        assertFalse(isIsolatable(ProcessCanceledException()))
        assertFalse(isIsolatable(CancellationException()))
        assertFalse(isIsolatable(InterruptedException()))
        assertFalse(isIsolatable(OutOfMemoryError()))
        assertTrue(isIsolatable(StackOverflowError()))
        assertTrue(isIsolatable(IllegalStateException()))
        assertTrue(isIsolatable(NotImplementedError()))

        val recorder = FirRuleErrorRecorder()
        val thrown = runCatching {
            isolateRule(recorder, "R", { "F.kt" }) { throw ProcessCanceledException() }
        }.exceptionOrNull()
        assertTrue(thrown is ProcessCanceledException)
        assertEquals(emptyMap(), recorder.snapshot())

        isolateRule(recorder, "R", { "F.kt" }) { error("first") }
        isolateRule(recorder, "R", { "F.kt" }) { error("second") }
        isolateRule(recorder, "R", { throw IllegalStateException("no path") }) { error("unknown file") }
        val recorded = recorder.snapshot().getValue("R")
        assertTrue("first" in recorded.getValue("F.kt"), recorded.toString())
        assertTrue("unknown file" in recorded.getValue(""), recorded.toString())
    }
}
