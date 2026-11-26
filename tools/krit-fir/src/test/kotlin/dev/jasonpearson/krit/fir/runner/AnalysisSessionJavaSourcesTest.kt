package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

/**
 * Kotlin that uses Java sources under the session's source dirs (same-module Java, generated Java such as view
 * binding and BuildConfig, Java in an extracted srcjar) resolves without a prior `compileJava` (#1626, #1630), and an
 * edit to that Java reaches every cache keyed on the compilation.
 */
class AnalysisSessionJavaSourcesTest {
    @TempDir lateinit var tmp: Path

    private val stdlib = File(Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
    private val sessions = ArrayList<AnalysisSession>()

    @AfterEach fun dispose() { sessions.forEach { it.dispose() } }

    private fun write(relative: String, text: String): File =
        tmp.resolve(relative).toFile().apply { parentFile.mkdirs(); writeText(text) }

    private fun session(vararg dirs: String, planned: Boolean = false): AnalysisSession =
        AnalysisSession(
            dirs.map { tmp.resolve(it).toString() }, listOf(stdlib),
            incrementalRecording = planned, incrementalSnapshot = planned, incrementalPlanning = planned,
        ).also { it.snapshotRootOverride = tmp.resolve("snapshots").toFile(); sessions += it }

    @Test fun kotlinResolvesSameModuleJava() {
        write("src/main/java/demo/j/Lookup.java", "package demo.j;\npublic final class Lookup { public static String find(String k) { return k; } }\n")
        val use = write("src/main/kotlin/demo/Use.kt", "package demo\nimport demo.j.Lookup\nfun f(k: String): String = Lookup.find(k)\n")

        val result = session("src/main/kotlin", "src/main/java").check(1, listOf(FileRef(use.path)), emptySet())

        assertTrue(result.errorFiles.isEmpty(), result.errorFiles.toString())
        assertEquals(1, result.succeeded)
    }

    @Test fun kotlinResolvesGeneratedJavaWhoseDirectoryDoesNotMatchItsPackage() {
        // Generated roots (an extracted srcjar, a processor's output) need not lay files out by package: each Java file
        // is its own root, which K2 indexes by the file's package header.
        write("gen/Msg.java", "package gen.wire;\npublic final class Msg { public String name() { return \"m\"; } }\n")
        write("gen/out/BuildConfig.java", "package com.example;\npublic final class BuildConfig { public static final String VERSION_NAME = \"1\"; }\n")
        val use = write(
            "src/main/kotlin/app/Uses.kt",
            "package app\nimport gen.wire.Msg\nimport com.example.BuildConfig\nfun f(): String = Msg().name() + BuildConfig.VERSION_NAME\n",
        )

        val result = session("src/main/kotlin", "gen").check(1, listOf(FileRef(use.path)), emptySet())

        assertTrue(result.errorFiles.isEmpty(), result.errorFiles.toString())
    }

    @Test fun javaFilesAreResolutionInputsNotCompiledSources() {
        write("src/main/java/demo/Lookup.java", "package demo;\npublic final class Lookup {}\n")
        write("src/main/kotlin/demo/Use.kt", "package demo\nfun f() = Lookup()\n")
        val session = session("src/main/kotlin", "src/main/java")

        assertEquals(listOf(tmp.resolve("src/main/kotlin/demo/Use.kt").toString()), session.currentSourceFiles())
        assertEquals(listOf(tmp.resolve("src/main/java/demo/Lookup.java").toString()), session.currentJavaSourceFiles())
    }

    @Test fun javaEditChangesTheCompileKey() {
        val java = write("src/main/java/demo/Lookup.java", "package demo;\npublic final class Lookup { public static String find() { return \"\"; } }\n")
        write("src/main/kotlin/demo/Use.kt", "package demo\nfun f() = Lookup.find()\n")
        val session = session("src/main/kotlin", "src/main/java")
        val before = session.compileKeyNow()

        // Same size, so only the content tells the two apart.
        java.writeText(java.readText().replace("find", "look"))
        val edited = session.compileKeyNow()
        assertNotEquals(before, edited, "a Java edit must change the compile key")

        write("src/main/java/demo/Extra.java", "package demo;\npublic final class Extra {}\n")
        assertNotEquals(edited, session.compileKeyNow(), "an added Java file must change the compile key")
    }

    @Test fun plannedCheckDoesNotServeAStaleVerdictAfterAJavaEdit() {
        val java = write("src/main/java/demo/Lookup.java", "package demo;\npublic final class Lookup { public static String find() { return \"\"; } }\n")
        val use = write("src/main/kotlin/demo/Use.kt", "package demo\nfun f(): String = Lookup.find()\n")
        val other = write("src/main/kotlin/demo/Other.kt", "package demo\nfun g() = 1\n")
        val session = session("src/main/kotlin", "src/main/java", planned = true)
        val files = listOf(FileRef(use.path), FileRef(other.path))

        val first = session.check(1, files, emptySet())
        assertTrue(first.errorFiles.isEmpty(), first.errorFiles.toString())
        assertTrue(session.incrementalState.snapshot != null, "a clean module gets a snapshot")

        // No Kotlin file changed; only the Java method Use.kt calls is gone.
        java.writeText(java.readText().replace("find", "look"))
        val second = session.check(2, files, emptySet())

        assertEquals(setOf(use.path), second.errorFiles.keys, second.toString())
        assertTrue(second.errorFiles.getValue(use.path).contains("find"), second.errorFiles.toString())
    }

    @Test fun sliceCompileStillResolvesJava() {
        write("src/main/java/demo/Lookup.java", "package demo;\npublic final class Lookup { public static String find() { return \"\"; } }\n")
        val use = write("src/main/kotlin/demo/Use.kt", "package demo\nfun f(): String = Lookup.find()\n")
        val other = write("src/main/kotlin/demo/Other.kt", "package demo\nfun g() = 1\n")
        val session = session("src/main/kotlin", "src/main/java", planned = true)
        session.check(1, listOf(FileRef(use.path), FileRef(other.path)), emptySet())

        use.writeText("package demo\nfun f(): String = Lookup.find() + \"!\"\n")
        val outcome = session.sliceCheck(2, listOf(FileRef(use.path)), emptySet())

        val checked = outcome as? dev.jasonpearson.krit.fir.incremental.SliceOutcome.Checked
            ?: error("slice fell back: $outcome")
        assertTrue(checked.result.errorFiles.isEmpty(), checked.result.errorFiles.toString())
    }
}
