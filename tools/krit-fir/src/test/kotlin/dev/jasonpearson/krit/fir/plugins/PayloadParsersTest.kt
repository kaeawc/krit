package dev.jasonpearson.krit.fir.plugins

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class PayloadParsersTest {

    @Test
    fun extractObjectBlockReturnsBraceBalancedSlice() {
        val json = """{"outer":"x","gradle":{"minSdk":21,"nested":{"k":"v"}},"after":"y"}"""
        val slice = PayloadParsers.extractObjectBlock(json, "gradle")
        // The whole gradle body should round-trip, including the
        // nested object — the brace counter must not bail on the
        // inner `}`.
        assertEquals("""{"minSdk":21,"nested":{"k":"v"}}""", slice)
    }

    @Test
    fun extractObjectBlockReturnsNullWhenMissingOrWrongType() {
        // Missing key.
        assertNull(PayloadParsers.extractObjectBlock("""{"x":1}""", "gradle"))
        // Wrong type — value is an array, not an object. The slicer
        // must refuse rather than returning a malformed substring.
        assertNull(PayloadParsers.extractObjectBlock("""{"gradle":[1,2]}""", "gradle"))
    }

    @Test
    fun extractStringHonoursJsonEscapeSet() {
        val json = """{"msg":"hello\nworld with a \"quote\" and a \\back\\"}"""
        assertEquals(
            "hello\nworld with a \"quote\" and a \\back\\",
            PayloadParsers.extractString(json, "msg"),
        )
    }

    @Test
    fun extractStringArraySupportsEmptyArrayAndEscapes() {
        assertEquals(emptyList(), PayloadParsers.extractStringArray("""{"deps":[]}""", "deps"))
        assertEquals(
            listOf("a", "b", "c with \"quote\""),
            PayloadParsers.extractStringArray("""{"deps":["a","b","c with \"quote\""]}""", "deps"),
        )
    }

    @Test
    fun extractStringArrayReturnsNullWhenMissing() {
        // Distinction between "absent" (null) and "present but empty"
        // (empty list) is load-bearing for the parser: empty payloads
        // ship a key with an empty body, so a missing key surfaces as
        // "the Go side didn't even include this fact".
        assertNull(PayloadParsers.extractStringArray("""{"other":[]}""", "deps"))
    }

    @Test
    fun extractLongHandlesNegativesAndAbsent() {
        assertEquals(42L, PayloadParsers.extractLong("""{"n":42}""", "n"))
        assertEquals(-7L, PayloadParsers.extractLong("""{"n":-7}""", "n"))
        assertNull(PayloadParsers.extractLong("""{"x":1}""", "n"))
    }

    @Test
    fun keyMatcherRejectsSubstringMatchesInOtherKeys() {
        // The lookup mustn't match `"k":"v"` when asked for `"key"`,
        // and vice versa. Substring-style key matching is a classic
        // hand-rolled-parser footgun.
        val json = """{"keyword":"steel","key":"target"}"""
        assertEquals("target", PayloadParsers.extractString(json, "key"))
        assertEquals("steel", PayloadParsers.extractString(json, "keyword"))
    }

    @Test
    fun projectPayloadsParseRoundTripsGradleBlock() {
        val json = """
            {"command":"analyzeFile","path":"/x.kt",
             "gradle":{"minSdk":21,"targetSdk":34,"agpVersion":"8.5.0",
                       "deps":["org.x:y:1.2.3","org.x:z:4.5.6"]}}
        """.trimIndent()
        val payloads = ProjectPayloads.parse(json)
        val gradle = payloads.gradle
        assertNotNull(gradle)
        assertEquals(21, gradle.minSdk)
        assertEquals(34, gradle.targetSdk)
        assertEquals("8.5.0", gradle.agpVersion)
        assertEquals(listOf("org.x:y:1.2.3", "org.x:z:4.5.6"), gradle.deps)
        // The other payload kinds aren't in the request → null.
        assertNull(payloads.manifest)
        assertNull(payloads.resources)
        assertNull(payloads.moduleIndex)
        assertNull(payloads.crossFile)
    }

    @Test
    fun projectPayloadsParsesAllFiveBlocks() {
        val json = """
            {"gradle":{"deps":["a:b:1"]},
             "manifest":{"package":"com.acme","permissions":["INTERNET"]},
             "resources":{"strings":["app_name=Acme"]},
             "moduleIndex":{"modules":[":app|/path/app||"]},
             "crossFile":{"declarations":["com.acme.Foo|class|/Foo.kt|10|public"]}}
        """.trimIndent()
        val payloads = ProjectPayloads.parse(json)
        assertNotNull(payloads.gradle)
        assertNotNull(payloads.manifest)
        assertNotNull(payloads.resources)
        assertNotNull(payloads.moduleIndex)
        assertNotNull(payloads.crossFile)
        assertEquals("com.acme", payloads.manifest.packageName)
        assertEquals(listOf("INTERNET"), payloads.manifest.permissions)
    }

    @Test
    fun emptyJsonProducesEmptyProjectPayloads() {
        val payloads = ProjectPayloads.parse("""{"command":"analyzeFile","path":"/x.kt"}""")
        assertTrue(payloads === ProjectPayloads.EMPTY || (
            payloads.gradle == null &&
                payloads.manifest == null &&
                payloads.resources == null &&
                payloads.moduleIndex == null &&
                payloads.crossFile == null
        ))
    }
}
