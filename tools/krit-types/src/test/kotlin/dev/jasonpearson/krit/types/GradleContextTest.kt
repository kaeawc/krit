package dev.jasonpearson.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class GradleContextTest {

    private fun ctx(
        minSdk: Int? = null,
        targetSdk: Int? = null,
        compileSdk: Int? = null,
        kotlinVersion: String? = null,
        javaTargetVersion: String? = null,
        agpVersion: String? = null,
        deps: List<String> = emptyList(),
    ) = PayloadGradleContext(
        GradleProfilePayload(
            minSdk = minSdk,
            targetSdk = targetSdk,
            compileSdk = compileSdk,
            kotlinVersion = kotlinVersion,
            javaTargetVersion = javaTargetVersion,
            agpVersion = agpVersion,
            deps = deps,
        ),
    )

    @Test
    fun scalarsPassThroughVerbatim() {
        val gradle = ctx(
            minSdk = 24,
            targetSdk = 34,
            compileSdk = 34,
            kotlinVersion = "2.0.0",
            javaTargetVersion = "17",
            agpVersion = "8.5.0",
        )
        assertEquals(24, gradle.minSdk)
        assertEquals(34, gradle.targetSdk)
        assertEquals(34, gradle.compileSdk)
        assertEquals("2.0.0", gradle.kotlinVersion)
        assertEquals("17", gradle.javaTargetVersion)
        assertEquals("8.5.0", gradle.agpVersion)
    }

    @Test
    fun absentScalarsRemainNull() {
        val gradle = ctx()
        assertNull(gradle.minSdk)
        assertNull(gradle.kotlinVersion)
    }

    @Test
    fun hasDependencyMatchesGroupAndName() {
        val gradle = ctx(deps = listOf("androidx.compose.ui:ui:1.6.0"))
        assertTrue(gradle.hasDependency("androidx.compose.ui", "ui"))
        assertFalse(gradle.hasDependency("androidx.compose.ui", "compiler"))
        assertFalse(gradle.hasDependency("androidx", "compose.ui:ui"))
    }

    @Test
    fun dependencyVersionReturnsTrailingSegment() {
        val gradle = ctx(deps = listOf("androidx.compose.ui:ui:1.6.0"))
        assertEquals("1.6.0", gradle.dependencyVersion("androidx.compose.ui", "ui"))
        assertNull(gradle.dependencyVersion("androidx.compose.ui", "compiler"))
    }

    @Test
    fun malformedDepEntriesAreIgnored() {
        // Defensive — a stray entry without three colon segments must
        // not corrupt the map or throw on parse. The Go-side caller
        // drops these, but rules shouldn't blow up if a future caller
        // forgets.
        val gradle = ctx(
            deps = listOf(
                "no-colons",
                "one:colon",
                ":empty-group:1.0",
                "group:empty-version:",
                "androidx.compose.ui:ui:1.6.0",
            ),
        )
        assertTrue(gradle.hasDependency("androidx.compose.ui", "ui"))
        assertFalse(gradle.hasDependency("group", "empty-version"))
    }

    @Test
    fun multiColonVersionPreservesEverythingAfterTheSecondColon() {
        // Maven version strings sometimes include qualifiers separated
        // by ':' under unusual packaging schemes. The contract is
        // "group:name:version" so versions with colons are unsupported,
        // but we should at least not lose data — preserve everything
        // after the second colon as the version.
        val gradle = ctx(deps = listOf("g:n:1.0.0:classifier"))
        assertEquals("1.0.0:classifier", gradle.dependencyVersion("g", "n"))
    }
}

class ExtractJsonObjectBlockTest {

    @Test
    fun extractsTopLevelObjectVerbatim() {
        val json = """{"id":1,"gradle":{"minSdk":24,"deps":["g:n:v"]},"trailing":true}"""
        val block = extractJsonObjectBlock(json, "gradle")
        assertEquals("""{"minSdk":24,"deps":["g:n:v"]}""", block)
    }

    @Test
    fun returnsNullWhenKeyAbsent() {
        assertNull(extractJsonObjectBlock("""{"id":1}""", "gradle"))
    }

    @Test
    fun balancesBracesInsideNestedObjects() {
        val json = """{"x":{"a":{"b":1},"c":2}}"""
        val block = extractJsonObjectBlock(json, "x")
        assertEquals("""{"a":{"b":1},"c":2}""", block)
    }

    @Test
    fun ignoresBracesInsideStrings() {
        val json = """{"x":{"k":"value with } brace"}}"""
        val block = extractJsonObjectBlock(json, "x")
        assertEquals("""{"k":"value with } brace"}""", block)
    }

    @Test
    fun parsesGradleProfileViaParseRequest() {
        val json = """{"id":1,"method":"analyzeFile","params":{"path":"X.kt","gradle":{"minSdk":24,"targetSdk":34,"compileSdk":34,"kotlinVersion":"2.0.0","agpVersion":"8.5.0","deps":["g:n:v","g2:n2:v2"]}}}"""
        val req = parseRequest(json)
        val gradle = req.gradleProfile ?: error("gradleProfile must round-trip from parseRequest")
        assertEquals(24, gradle.minSdk)
        assertEquals(34, gradle.targetSdk)
        assertEquals(34, gradle.compileSdk)
        assertEquals("2.0.0", gradle.kotlinVersion)
        assertEquals("8.5.0", gradle.agpVersion)
        assertEquals(listOf("g:n:v", "g2:n2:v2"), gradle.deps)
    }

    @Test
    fun parseRequestLeavesGradleProfileNullWhenAbsent() {
        val json = """{"id":1,"method":"analyzeFile","params":{"path":"X.kt"}}"""
        assertNull(parseRequest(json).gradleProfile)
    }
}
