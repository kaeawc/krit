package dev.jasonpearson.krit.fir.tests

import org.jetbrains.kotlin.cli.common.ExitCode
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import java.io.File
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

// Guards the probe's "did this compile cleanly" verdict, which every negative
// case relies on: a compile that fails without an error in a requested source
// must still be reported as not clean.
class KritFirProbeTest {

    private val source = mapOf("Ok.kt" to "package ok\nfun ok() {}\n")

    @Test
    fun cleanCompileIsClean() {
        val result = KritFirProbe.compile(source)
        assertEquals(ExitCode.OK, result.exitCode)
        assertTrue(result.clean, result.problems())
    }

    // A missing source root is reported without a location; before the exit
    // code was checked this compile counted as clean with zero findings.
    @Test
    fun locationLessErrorIsNotClean() {
        val missing = File(kotlin.io.path.createTempDirectory("krit-fir-probe-test").toFile(), "missing")
        val result = KritFirProbe.compile(source) { it.freeArgs = it.freeArgs + missing.path }
        assertFalse(result.clean, "a failed compile was reported clean")
        assertTrue(result.compileErrors.isEmpty(), "the error is not located in a requested source")
        assertTrue(result.otherErrors.any { it.startsWith("(no location)") }, result.problems())
        assertTrue("(no location)" in result.problems(), result.problems())
    }

    @Test
    fun diagnoseFailsOnLocatedError() {
        val error = assertThrows<IllegalStateException> {
            KritFirProbe.diagnose(mapOf("Bad.kt" to "package bad\nfun f() { missingCall() }\n"))
        }
        assertTrue("Bad.kt:2" in error.message.orEmpty(), error.message)
    }
}
