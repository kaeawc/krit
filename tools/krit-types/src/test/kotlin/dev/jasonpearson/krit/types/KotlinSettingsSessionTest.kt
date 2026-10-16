package dev.jasonpearson.krit.types

import com.intellij.openapi.util.Disposer
import java.nio.file.Files
import java.nio.file.Path
import kotlin.io.path.createTempDirectory
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * The same source analyzed through the Kotlin Analysis API under different project
 * settings must resolve the way that project's compiler would, not the embedded
 * 2.4 defaults. The annotation-target default is the observable difference: on a
 * constructor `val`, 2.4 applies the annotation to the parameter and the property,
 * older language levels to the parameter only.
 */
class KotlinSettingsSessionTest {
    private val stdlib = Path.of(KotlinVersion::class.java.protectionDomain.codeSource.location.toURI()).toString()

    private fun export(source: String, kotlinArgs: List<String>): String {
        val root = createTempDirectory("krit-kaa-settings-")
        root.toFile().deleteOnExit()
        Files.writeString(root.resolve("A.kt"), source)
        val disposable = Disposer.newDisposable("kotlin-settings-test")
        try {
            return analyzeAndExport(
                disposable,
                ParsedArgs(sourceDirs = listOf(root.toString()), classpath = listOf(stdlib), jdkHome = null, output = null, kotlinArgs = kotlinArgs),
            )
        } finally {
            Disposer.dispose(disposable)
        }
    }

    private val annotationTarget = """
        package p
        @Target(AnnotationTarget.VALUE_PARAMETER, AnnotationTarget.PROPERTY)
        annotation class Tag
        class Box(@Tag val label: String)
    """.trimIndent()

    private fun labelPropertyAnnotations(kotlinArgs: List<String>): String {
        val json = export(annotationTarget, kotlinArgs)
        val member = Regex(""""name": "label", "kind": "property"[^}]*\}""").find(json)?.value
            ?: error("no label property in export: $json")
        return Regex(""""annotations": \[([^\]]*)]""").find(member)?.groupValues?.get(1).orEmpty()
    }

    @Test
    fun annotationTargetDefaultFollowsTheProjectsLanguageLevel() {
        assertEquals("\"p.Tag\"", labelPropertyAnnotations(emptyList()), "embedded 2.4 default")
        for (level in listOf("2.1", "2.2", "2.3")) {
            assertEquals("", labelPropertyAnnotations(listOf("-language-version=$level", "-api-version=$level")),
                "a project pinned to $level keeps first-only annotation targets")
        }
        assertEquals("", labelPropertyAnnotations(listOf("-Xannotation-default-target=first-only")))
        assertEquals("\"p.Tag\"", labelPropertyAnnotations(listOf("-language-version=2.3", "-api-version=2.3",
            "-Xannotation-default-target=param-property")), "the allowlisted flag re-enables the new behavior on the old level")
    }

    @Test
    fun settingsOutsideTheAllowlistLeaveAnalysisUnchanged() {
        val ignored = listOf("-d", "/nonexistent/out", "-classpath=/evil.jar", "-Xplugin=/evil.jar", "-Werror", "-language-version=1.9")
        assertEquals("\"p.Tag\"", labelPropertyAnnotations(ignored))
        assertTrue(KotlinCompilerSettings.parse(ignored).warnings.size == ignored.size - 1, "every argument is reported")
    }
}
