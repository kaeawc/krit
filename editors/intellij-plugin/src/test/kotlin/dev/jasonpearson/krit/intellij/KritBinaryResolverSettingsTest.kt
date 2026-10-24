package dev.jasonpearson.krit.intellij

import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class KritBinaryResolverSettingsTest {
    @Test
    fun `settings override wins over system property`() {
        val settingsBinary = makeExecutable("from-settings")
        val propBinary = makeExecutable("from-prop")
        val found = KritBinaryResolver.find(
            env = mapOf("PATH" to ""),
            props = { propBinary.absolutePath },
            settings = { settingsBinary.absolutePath },
        )
        assertEquals(settingsBinary.canonicalFile, found?.canonicalFile)
    }

    @Test
    fun `settings override wins over env`() {
        val settingsBinary = makeExecutable("from-settings")
        val envBinary = makeExecutable("from-env")
        val found = KritBinaryResolver.find(
            env = mapOf("KRIT_BINARY" to envBinary.absolutePath, "PATH" to ""),
            props = { null },
            settings = { settingsBinary.absolutePath },
        )
        assertEquals(settingsBinary.canonicalFile, found?.canonicalFile)
    }

    @Test
    fun `blank settings falls through to system property`() {
        // Empty string in settings is the "unset" sentinel — don't treat
        // it as an explicit override, fall through to the next source.
        val propBinary = makeExecutable("from-prop")
        val found = KritBinaryResolver.find(
            env = mapOf("PATH" to ""),
            props = { propBinary.absolutePath },
            settings = { "" },
        )
        assertEquals(propBinary.canonicalFile, found?.canonicalFile)
    }

    @Test
    fun `non-executable settings path returns null without falling through`() {
        // Honors the explicit user choice: if the user pointed at a path
        // through Settings, that's their answer. Silently fall-through to
        // PATH would hide the misconfiguration.
        val nonExec = Files.createTempFile("krit-settings-not-exec", ".txt").toFile()
        nonExec.deleteOnExit()
        val pathBinary = makeExecutable("krit")
        val found = KritBinaryResolver.find(
            env = mapOf("PATH" to pathBinary.parentFile.absolutePath),
            props = { null },
            settings = { nonExec.absolutePath },
        )
        assertNull(found)
    }

    @Test
    fun `project override wins over the application setting`() {
        val projectBinary = makeExecutable("from-project")
        val appBinary = makeExecutable("from-settings")
        val found = KritBinaryResolver.find(
            env = mapOf("PATH" to ""),
            props = { null },
            settings = { appBinary.absolutePath },
            projectSettings = { projectBinary.absolutePath },
        )
        assertEquals(projectBinary.canonicalFile, found?.canonicalFile)
    }

    @Test
    fun `blank project override falls through to the application setting`() {
        val appBinary = makeExecutable("from-settings")
        val found = KritBinaryResolver.find(
            env = mapOf("PATH" to ""),
            props = { null },
            settings = { appBinary.absolutePath },
            projectSettings = { " " },
        )
        assertEquals(appBinary.canonicalFile, found?.canonicalFile)
    }

    @Test
    fun `non-executable project override returns null without falling through`() {
        val nonExec = Files.createTempFile("krit-project-not-exec", ".txt").toFile()
        nonExec.deleteOnExit()
        val appBinary = makeExecutable("from-settings")
        val found = KritBinaryResolver.find(
            env = mapOf("PATH" to ""),
            props = { null },
            settings = { appBinary.absolutePath },
            projectSettings = { nonExec.absolutePath },
        )
        assertNull(found)
    }

    private fun makeExecutable(name: String): File {
        val dir = Files.createTempDirectory("krit-settings-$name").toFile()
        dir.deleteOnExit()
        val f = File(dir, name)
        f.writeText("#!/bin/sh\nexit 0\n")
        f.setExecutable(true)
        f.deleteOnExit()
        return f
    }
}
