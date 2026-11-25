package dev.jasonpearson.krit.gradle

import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

class KritModelInputsTest {
    @TempDir lateinit var directory: File

    private fun model(root: String, sibling: Boolean = true, name: String = "lib.json"): File = File(directory, "${root.hashCode()}/$name").apply {
        parentFile.mkdirs()
        writeText(
            """
            {"schema": 2, "rootDir": "$root", "projects": [{"path": ":lib", "dir": "$root/lib",
              "sourceSets": [{"name": "main", "sourceDirs": ["$root/lib/src/main/kotlin"${if (sibling) ", \"${root}2/elsewhere\"" else ""}],
                "classpath": ["/home/a/.gradle/caches/x/dep-1.0.jar", "$root/lib/libs/local.jar"],
                "bootClasspath": [], "generatedClasspath": ["$root/lib/build/R.jar"],
                "kotlin": {"compilerPluginClasspath": ["/opt/plugin.jar"], "optIn": ["a.B"]}}]}]}
            """.trimIndent()
        )
    }

    @Test
    fun `paths under the root are relative and classpath entries are names`() {
        val normalized = KritModelInputs.normalized(listOf(model("/repo")))
        assertTrue(normalized.contains("\"dir\":\"\$ROOT/lib\""), normalized)
        assertTrue(normalized.contains("\"rootDir\":\"\$ROOT\""), normalized)
        assertTrue(normalized.contains("\"\$ROOT/lib/src/main/kotlin\""), normalized)
        // A sibling directory that merely shares the root's prefix stays absolute.
        assertTrue(normalized.contains("\"/repo2/elsewhere\""), normalized)
        assertTrue(normalized.contains("\"classpath\":[\"dep-1.0.jar\",\"local.jar\"]"), normalized)
        assertTrue(normalized.contains("\"compilerPluginClasspath\":[\"plugin.jar\"]"), normalized)
        assertFalse(normalized.contains("/home/a/.gradle"), normalized)
    }

    @Test
    fun `the same model at another root normalizes identically`() {
        assertEquals(
            KritModelInputs.normalized(listOf(model("/repo", sibling = false))),
            KritModelInputs.normalized(listOf(model("/ci/work/repo", sibling = false))),
        )
    }

    @Test
    fun `classpath lists every classpath entry the model holds`() {
        assertEquals(
            listOf("/home/a/.gradle/caches/x/dep-1.0.jar", "/repo/lib/libs/local.jar", "/repo/lib/build/R.jar", "/opt/plugin.jar"),
            KritModelInputs.classpath(listOf(model("/repo"))).map { it.path },
        )
    }
}
