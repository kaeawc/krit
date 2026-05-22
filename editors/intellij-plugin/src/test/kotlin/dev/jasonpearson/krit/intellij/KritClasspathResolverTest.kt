package dev.jasonpearson.krit.intellij

import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals

class KritClasspathResolverTest {
    @Test
    fun `toClasspathString joins on platform separator`() {
        val a = File("/path/to/a.jar")
        val b = File("/path/to/b.jar")
        assertEquals(
            listOf(a.absolutePath, b.absolutePath).joinToString(File.pathSeparator),
            KritClasspathResolver.toClasspathString(listOf(a, b)),
        )
    }

    @Test
    fun `toClasspathString returns empty for empty list`() {
        // Empty CLASSPATH is what splitEnvClasspath treats as no
        // classpath; matters so the runner can skip setting the env
        // var entirely without ambiguity.
        assertEquals("", KritClasspathResolver.toClasspathString(emptyList()))
    }

    @Test
    fun `toClasspathString preserves order and duplicates`() {
        // Dedup happens earlier in resolve(); toClasspathString is a
        // pure join — keep it that way so unit tests can pin behaviour
        // without bringing up an IntelliJ project model.
        val a = File("/a.jar")
        val b = File("/b.jar")
        val result = KritClasspathResolver.toClasspathString(listOf(a, b, a))
        val parts = result.split(File.pathSeparator)
        assertEquals(3, parts.size)
        assertEquals(a.absolutePath, parts[0])
        assertEquals(b.absolutePath, parts[1])
        assertEquals(a.absolutePath, parts[2])
    }
}
