package dev.jasonpearson.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ModuleIndexContextTest {

    private fun ctx(vararg entries: String) = PayloadModuleIndexContext(ModulesProfilePayload(entries.toList()))

    @Test
    fun modulePathsAreDiscoveredInWireOrder() {
        val moduleIndex = ctx(
            ":app|/abs/app||",
            ":core:util|/abs/core/util||",
        )
        assertEquals(listOf(":app", ":core:util"), moduleIndex.modulePaths)
    }

    @Test
    fun directoryAndDependsOnAreParsedFromPipeDelimitedEntry() {
        val moduleIndex = ctx(":app|/abs/app|:core,:data|/abs/app/src/main/kotlin")
        assertEquals("/abs/app", moduleIndex.directoryOf(":app"))
        assertEquals(listOf(":core", ":data"), moduleIndex.dependenciesOf(":app"))
        assertEquals(listOf("/abs/app/src/main/kotlin"), moduleIndex.sourceRootsOf(":app"))
    }

    @Test
    fun missingModuleReturnsNullDirectoryAndEmptyLists() {
        val moduleIndex = ctx(":app|/abs/app||")
        assertNull(moduleIndex.directoryOf(":missing"))
        assertTrue(moduleIndex.dependenciesOf(":missing").isEmpty())
        assertTrue(moduleIndex.sourceRootsOf(":missing").isEmpty())
    }

    @Test
    fun moduleWithNoDependenciesOrSourceRootsReturnsEmptyLists() {
        val moduleIndex = ctx(":core|/abs/core||")
        assertTrue(moduleIndex.dependenciesOf(":core").isEmpty())
        assertTrue(moduleIndex.sourceRootsOf(":core").isEmpty())
    }

    @Test
    fun malformedEntriesAreIgnoredNotThrown() {
        // Defensive — a stray entry without a pipe must not corrupt the
        // map or throw on lookup. Go-side caller emits well-formed
        // entries; this guards against future regressions.
        val moduleIndex = ctx("no-pipe", "|empty-path", ":app|/abs/app||")
        assertEquals(listOf(":app"), moduleIndex.modulePaths)
    }

    @Test
    fun parsesModulesProfileViaParseRequest() {
        val json = """{"id":1,"method":"analyzeFile","params":{"path":"X.kt","moduleIndex":{"modules":[":app|/abs/app|:core|/abs/app/src/main/kotlin"]}}}"""
        val req = parseRequest(json)
        val modules = req.modulesProfile ?: error("modulesProfile must round-trip")
        assertEquals(listOf(":app|/abs/app|:core|/abs/app/src/main/kotlin"), modules.modules)
    }

    @Test
    fun parseRequestLeavesModulesProfileNullWhenAbsent() {
        val json = """{"id":1,"method":"analyzeFile","params":{"path":"X.kt"}}"""
        assertNull(parseRequest(json).modulesProfile)
    }
}
