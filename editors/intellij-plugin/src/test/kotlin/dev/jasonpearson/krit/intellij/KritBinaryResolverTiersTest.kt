package dev.jasonpearson.krit.intellij

import dev.jasonpearson.krit.intellij.KritBinaryResolver.Resolution
import dev.jasonpearson.krit.intellij.KritBinaryResolver.Source
import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertSame

class KritBinaryResolverTiersTest {
    private val downloaded = makeExecutable("krit", "downloaded")

    @Test
    fun `a downloaded binary is used only when nothing else is found`() {
        val resolution = KritBinaryResolver.resolve(env = mapOf("PATH" to ""), props = { null }, settings = { null }, downloaded = { downloaded })

        val found = assertIs<Resolution.Found>(resolution)
        assertEquals(Source.DOWNLOADED, found.source)
        assertEquals(downloaded, found.binary)
    }

    @Test
    fun `krit on PATH wins over a downloaded binary`() {
        val onPath = makeExecutable("krit", "path")
        val resolution = KritBinaryResolver.resolve(
            env = mapOf("PATH" to onPath.parentFile.absolutePath),
            props = { null },
            settings = { null },
            downloaded = { downloaded },
        )

        assertEquals(Resolution.Found(onPath, Source.PATH), resolution)
    }

    @Test
    fun `settings and KRIT_BINARY win over PATH and a download`() {
        val fromSettings = makeExecutable("custom-krit", "settings")
        val fromEnv = makeExecutable("env-krit", "env")
        val onPath = makeExecutable("krit", "path")
        val env = mapOf("KRIT_BINARY" to fromEnv.absolutePath, "PATH" to onPath.parentFile.absolutePath)

        assertEquals(
            Resolution.Found(fromSettings, Source.SETTINGS),
            KritBinaryResolver.resolve(env = env, props = { null }, settings = { fromSettings.absolutePath }, downloaded = { downloaded }),
        )
        assertEquals(
            Resolution.Found(fromEnv, Source.ENVIRONMENT),
            KritBinaryResolver.resolve(env = env, props = { null }, settings = { "" }, downloaded = { downloaded }),
        )
    }

    @Test
    fun `a broken configured path is reported, not replaced by PATH or a download`() {
        var downloadedAsked = false
        val resolution = KritBinaryResolver.resolve(
            env = mapOf("PATH" to makeExecutable("krit", "path").parentFile.absolutePath),
            props = { null },
            settings = { "/no/such/krit" },
            downloaded = { downloadedAsked = true; downloaded },
        )

        assertEquals(Resolution.ConfiguredMissing("/no/such/krit", Source.SETTINGS), resolution)
        assertEquals(false, downloadedAsked)
    }

    @Test
    fun `nothing anywhere is NotFound, the case that offers a download`() {
        val resolution = KritBinaryResolver.resolve(env = mapOf("PATH" to ""), props = { null }, settings = { null }, downloaded = { null })

        assertSame(Resolution.NotFound, resolution)
    }

    @Test
    fun `finds krit_exe on PATH on Windows`() {
        val exe = makeExecutable("krit.exe", "win")
        val resolution = KritBinaryResolver.resolve(
            env = mapOf("PATH" to exe.parentFile.absolutePath),
            props = { null },
            settings = { null },
            downloaded = { null },
            windows = true,
        )

        assertEquals(Resolution.Found(exe, Source.PATH), resolution)
    }

    private fun makeExecutable(name: String, dirNameHint: String): File {
        val dir = Files.createTempDirectory("krit-tiers-$dirNameHint").toFile()
        dir.deleteOnExit()
        val f = File(dir, name)
        f.writeText("#!/bin/sh\nexit 0\n")
        f.setExecutable(true)
        f.deleteOnExit()
        return f
    }
}
