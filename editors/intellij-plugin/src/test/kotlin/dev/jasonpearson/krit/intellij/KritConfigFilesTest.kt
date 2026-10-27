package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class KritConfigFilesTest {
    @Test
    fun `filenames mirror config Filenames in probe order`() {
        assertEquals(listOf("krit.yml", ".krit.yml"), KritConfigFiles.FILENAMES)
    }

    @Test
    fun `matches both config names at the project root`() {
        assertTrue(KritConfigFiles.isProjectConfig("/repo", "/repo/krit.yml"))
        assertTrue(KritConfigFiles.isProjectConfig("/repo", "/repo/.krit.yml"))
        assertTrue(KritConfigFiles.isProjectConfig("/repo/", "/repo/krit.yml"))
    }

    @Test
    fun `ignores config files below or beside the root`() {
        assertFalse(KritConfigFiles.isProjectConfig("/repo", "/repo/app/krit.yml"))
        assertFalse(KritConfigFiles.isProjectConfig("/repo", "/other/krit.yml"))
        assertFalse(KritConfigFiles.isProjectConfig("/repo", "/repo2/krit.yml"))
    }

    @Test
    fun `ignores other names`() {
        assertFalse(KritConfigFiles.isProjectConfig("/repo", "/repo/krit.yaml"))
        assertFalse(KritConfigFiles.isProjectConfig("/repo", "/repo/krit.yml.bak"))
        assertFalse(KritConfigFiles.isProjectConfig("/repo", "/repo/detekt.yml"))
    }

    @Test
    fun `no base path matches nothing`() {
        assertFalse(KritConfigFiles.isProjectConfig(null, "/repo/krit.yml"))
        assertFalse(KritConfigFiles.isProjectConfig("", "/repo/krit.yml"))
    }

    @Test
    fun `anyProjectConfig covers a rename onto the config name`() {
        // A rename event reports the old path and the new one.
        assertTrue(KritConfigFiles.anyProjectConfig("/repo", listOf("/repo/krit.yml.tmp", "/repo/krit.yml")))
        assertFalse(KritConfigFiles.anyProjectConfig("/repo", listOf("/repo/a.kt", null)))
    }
}
