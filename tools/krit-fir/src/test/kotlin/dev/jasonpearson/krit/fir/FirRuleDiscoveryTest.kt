package dev.jasonpearson.krit.fir

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.PrintStream
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

class FirRuleDiscoveryTest {
    @TempDir lateinit var tmp: Path

    private val testClasses = File(FirRuleDiscoveryTest::class.java.protectionDomain.codeSource.location.toURI())
    private val loader = FirRuleDiscoveryTest::class.java.classLoader
    private val goodPrefix = "dev/jasonpearson/krit/fir/discoveryfixtures/good/"
    private val badPrefix = "dev/jasonpearson/krit/fir/discoveryfixtures/bad/"

    @Test fun privateAndNestedObjectsAreDiscoveredAndAbstractBasesSkipped() {
        val ids = FirRuleDiscovery.discover(listOf(testClasses), loader, goodPrefix).map { it.ruleId }
        assertEquals(listOf("DiscoveryNestedObject", "DiscoveryPrivateObject"), ids)
    }

    @Test fun nonObjectImplementerFailsLoudlyNamingTheClass() {
        val error = assertFailsWith<IllegalStateException> {
            FirRuleDiscovery.discover(listOf(testClasses), loader, badPrefix)
        }
        assertTrue(
            "dev.jasonpearson.krit.fir.discoveryfixtures.bad.NotAnObjectRule" in error.message.orEmpty(),
            error.message,
        )
    }

    @Test fun unreadableJarOnTheScanPathIsSkipped() {
        val broken = tmp.resolve("broken.jar").toFile().apply { writeBytes(byteArrayOf(1, 2, 3, 4, 5)) }
        val stderr = ByteArrayOutputStream()
        val previous = System.err
        System.setErr(PrintStream(stderr, true))
        val ids = try {
            FirRuleDiscovery.discover(listOf(broken, testClasses), loader, goodPrefix).map { it.ruleId }
        } finally {
            System.setErr(previous)
        }
        assertEquals(listOf("DiscoveryNestedObject", "DiscoveryPrivateObject"), ids)
        assertTrue(broken.path in stderr.toString(), stderr.toString())
    }
}
