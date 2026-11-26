package dev.jasonpearson.krit.gradle

import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertNotEquals
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File

class KritModelInputsTest {
    @TempDir lateinit var directory: File

    private fun dep(home: String) = "$home/.gradle/caches/modules-2/files-2.1/g/dep/1.0/abc123/dep-1.0.jar"

    private fun model(
        root: String,
        home: String = "/home/a",
        sibling: Boolean = true,
        mainJar: String = "$root/lib/build/a/classes.jar",
        testJar: String = "$root/lib/build/b/classes.jar",
    ): File = File(directory, "${(root + home + mainJar).hashCode()}/lib.json").apply {
        parentFile.mkdirs()
        writeText(
            """
            {"schema": 2, "rootDir": "$root", "projects": [{"path": ":lib", "dir": "$root/lib",
              "sourceSets": [
                {"name": "main", "sourceDirs": ["$root/lib/src/main/kotlin"${if (sibling) ", \"${root}2/elsewhere\"" else ""}],
                 "classpath": ["${dep(home)}", "$root/lib/libs/local.jar", "$mainJar"],
                 "bootClasspath": [], "generatedClasspath": ["$root/lib/build/R.jar"],
                 "kotlin": {"compilerPluginClasspath": ["/opt/plugin.jar"], "optIn": ["a.B"]}},
                {"name": "test", "sourceDirs": [], "classpath": ["$testJar"]}]}]}
            """.trimIndent()
        )
    }

    @Test
    fun `paths under the root and the Gradle caches are location independent`() {
        val normalized = KritModelInputs.normalized(listOf(model("/repo")))
        assertTrue(normalized.contains("\"dir\":\"\$ROOT/lib\""), normalized)
        assertTrue(normalized.contains("\"rootDir\":\"\$ROOT\""), normalized)
        assertTrue(normalized.contains("\"\$ROOT/lib/src/main/kotlin\""), normalized)
        // A sibling directory that merely shares the root's prefix stays absolute.
        assertTrue(normalized.contains("\"/repo2/elsewhere\""), normalized)
        assertTrue(normalized.contains(
            "\"classpath\":[\"\$GRADLE_CACHES/modules-2/files-2.1/g/dep/1.0/abc123/dep-1.0.jar\"," +
                "\"\$ROOT/lib/libs/local.jar\",\"\$ROOT/lib/build/a/classes.jar\"]"), normalized)
        // Outside the root and the Gradle caches an entry keeps its absolute path.
        assertTrue(normalized.contains("\"compilerPluginClasspath\":[\"/opt/plugin.jar\"]"), normalized)
        assertFalse(normalized.contains("/home/a/.gradle"), normalized)
    }

    @Test
    fun `the same model at another root and Gradle home normalizes identically`() {
        assertEquals(
            KritModelInputs.normalized(listOf(model("/repo", sibling = false))),
            KritModelInputs.normalized(listOf(model("/ci/work/repo", home = "/home/runner", sibling = false))),
        )
    }

    // Jar contents are keyed by the @Classpath input over all models; which source
    // set uses which jar must stay in the normalized model, or swapping two
    // same-named jars (Android classes.jar) would keep the key.
    @Test
    fun `swapping same-named jars between source sets changes the key`() {
        val one = model("/repo", mainJar = "/repo/lib/build/a/classes.jar", testJar = "/repo/lib/build/b/classes.jar")
        val swapped = model("/repo", mainJar = "/repo/lib/build/b/classes.jar", testJar = "/repo/lib/build/a/classes.jar")
        assertNotEquals(KritModelInputs.normalized(listOf(one)), KritModelInputs.normalized(listOf(swapped)))
    }

    @Test
    fun `classpath lists every classpath entry the model holds`() {
        assertEquals(
            listOf(dep("/home/a"), "/repo/lib/libs/local.jar", "/repo/lib/build/a/classes.jar", "/repo/lib/build/R.jar",
                "/opt/plugin.jar", "/repo/lib/build/b/classes.jar"),
            KritModelInputs.classpath(listOf(model("/repo"))).map { it.path },
        )
    }

    // #1645: krit loads the R jar AGP wrote for the component, which may sit next to the prediction.
    @Test
    fun `a predicted R jar brings the directories AGP writes R jars to`() {
        val file = File(directory, "r/app.json").apply {
            parentFile.mkdirs()
            writeText("""{"schema": 2, "rootDir": "/repo", "projects": [{"path": ":app", "dir": "/repo/app", "sourceSets": [
              {"name": "debug", "classpath": [], "generatedClasspath": [
                "/repo/app/build/intermediates/compile_and_runtime_not_namespaced_r_class_jar/debug/processDebugResources/R.jar"]},
              {"name": "main", "classpath": [], "generatedClasspath": [
                "/repo/lib/build/intermediates/compile_r_class_jar/debug/generateDebugRFile/R.jar", "/repo/app/build/R.jar"]}]}]}""")
        }
        assertEquals(
            listOf(
                "/repo/app/build/intermediates/compile_and_runtime_not_namespaced_r_class_jar/debug/processDebugResources/R.jar",
                "/repo/app/build/intermediates/compile_and_runtime_not_namespaced_r_class_jar/debug",
                "/repo/app/build/intermediates/compile_and_runtime_r_class_jar/debug",
                "/repo/lib/build/intermediates/compile_r_class_jar/debug/generateDebugRFile/R.jar",
                "/repo/lib/build/intermediates/compile_r_class_jar/debug",
                "/repo/app/build/R.jar",
            ),
            KritModelInputs.classpath(listOf(file)).map { it.path },
        )
    }
}
