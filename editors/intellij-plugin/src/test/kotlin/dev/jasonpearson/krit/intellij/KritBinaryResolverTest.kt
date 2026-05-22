package dev.jasonpearson.krit.intellij

import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class KritBinaryResolverTest {
    @Test
    fun `system property overrides environment when both point to executable`() {
        val systemProp = makeExecutable("from-prop")
        val envOverride = makeExecutable("from-env")
        val found = KritBinaryResolver.find(
            env = mapOf("KRIT_BINARY" to envOverride.absolutePath, "PATH" to ""),
            props = { systemProp.absolutePath },
        )
        assertEquals(systemProp.canonicalFile, found?.canonicalFile)
    }

    @Test
    fun `env override is honored when no system property`() {
        val configured = makeExecutable("from-env")
        val found = KritBinaryResolver.find(
            env = mapOf("KRIT_BINARY" to configured.absolutePath, "PATH" to ""),
            props = { null },
        )
        assertEquals(configured.canonicalFile, found?.canonicalFile)
    }

    @Test
    fun `configured path that is not executable returns null even when PATH has one`() {
        // Honor the explicit override exactly: if the user pointed at a
        // path, that's the binary, not whatever happens to live on PATH.
        val notExecutable = Files.createTempFile("krit-not-exec", ".txt").toFile()
        notExecutable.deleteOnExit()
        val pathBinary = makeExecutable("krit")
        val found = KritBinaryResolver.find(
            env = mapOf("KRIT_BINARY" to notExecutable.absolutePath, "PATH" to pathBinary.parentFile.absolutePath),
            props = { null },
        )
        assertNull(found)
    }

    @Test
    fun `PATH lookup finds krit in first matching directory`() {
        val first = makeExecutable("krit", dirNameHint = "first")
        val second = makeExecutable("krit", dirNameHint = "second")
        val pathEntry = listOf(first.parentFile.absolutePath, second.parentFile.absolutePath)
            .joinToString(File.pathSeparator)
        val found = KritBinaryResolver.find(env = mapOf("PATH" to pathEntry), props = { null })
        assertEquals(first.canonicalFile, found?.canonicalFile)
    }

    @Test
    fun `returns null when no PATH and no override`() {
        assertNull(KritBinaryResolver.find(env = emptyMap(), props = { null }))
    }

    @Test
    fun `skips blank PATH entries`() {
        val executable = makeExecutable("krit")
        val pathEntry = listOf("", executable.parentFile.absolutePath, "")
            .joinToString(File.pathSeparator)
        val found = KritBinaryResolver.find(env = mapOf("PATH" to pathEntry), props = { null })
        assertEquals(executable.canonicalFile, found?.canonicalFile)
    }

    private fun makeExecutable(name: String, dirNameHint: String = "bin"): File {
        val dir = Files.createTempDirectory("krit-test-$dirNameHint").toFile()
        dir.deleteOnExit()
        val f = File(dir, name)
        f.writeText("#!/bin/sh\nexit 0\n")
        f.setExecutable(true)
        f.deleteOnExit()
        return f
    }
}
