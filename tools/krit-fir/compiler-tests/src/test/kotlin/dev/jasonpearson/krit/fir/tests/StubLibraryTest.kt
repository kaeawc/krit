package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import java.io.File
import java.io.StringWriter
import javax.tools.ToolProvider
import kotlin.test.assertNotNull
import kotlin.test.assertTrue
import kotlin.test.fail

// Structural guards for the shared source stubs: Kotlin stubs in
// src/test/data/stubs/*.kt and the Java platform layer in
// src/test/data/stubs/java/**.java.
class StubLibraryTest {

    private val stubsDir = File("src/test/data/stubs")
    private val javaStubsDir = stubsDir.resolve("java")
    private val smokeDir = File("src/test/data/diagnostic/stubs")
    private val packageRe = Regex("""^package\s+([\w.]+)""", RegexOption.MULTILINE)

    private fun kotlinStubs(): List<File> =
        stubsDir.listFiles { f -> f.extension == "kt" }.orEmpty().sortedBy { it.name }

    private fun javaStubs(): List<File> =
        javaStubsDir.walkTopDown().filter { it.isFile && it.extension == "java" }.sortedBy { it.path }.toList()

    private fun packageOf(file: File): String? = packageRe.find(file.readText())?.groupValues?.get(1)

    // KritFirProbe only reports compiler errors located in *requested* sources,
    // so an error inside a Kotlin stub (a bad override, a redeclaration) would
    // stay invisible unless some smoke file happened to trip over it.
    // Requesting every Kotlin stub under its own file name makes the probe treat
    // each one as a requested source: any stub error fails here, and no krit
    // rule may fire on a stub declaration.
    @Test
    fun kotlinStubsCompileCleanly() {
        val stubs = kotlinStubs()
        assertTrue(stubs.isNotEmpty(), "no stubs found in ${stubsDir.path}")
        val diags = KritFirProbe.diagnose(stubs.associate { it.name to it.readText() })
        if (diags.isNotEmpty()) fail("krit rules fired on stub declarations:\n" + diags.joinToString("\n"))
    }

    // kotlinc resolves the Java layer from source without javac and does not
    // report Java parse or type errors, so compile the Java layer on its own
    // with javac. It must be self-contained (JDK only): Java stubs never
    // reference Kotlin stubs.
    @Test
    fun javaStubsCompileWithJavac() {
        val sources = javaStubs()
        assertTrue(sources.isNotEmpty(), "no Java stubs found in ${javaStubsDir.path}")
        val compiler = assertNotNull(ToolProvider.getSystemJavaCompiler(), "tests must run on a JDK with javac")
        val outDir = kotlin.io.path.createTempDirectory("krit-fir-java-stubs").toFile()
        try {
            val output = StringWriter()
            val fileManager = compiler.getStandardFileManager(null, null, null)
            val units = fileManager.getJavaFileObjectsFromFiles(sources)
            val ok = compiler
                .getTask(output, fileManager, null, listOf("-proc:none", "-d", outDir.absolutePath), null, units)
                .call()
            fileManager.close()
            if (!ok) fail("Java stubs failed to compile with javac:\n$output")
        } finally {
            outDir.deleteRecursively()
        }
    }

    // Each Kotlin stub X.kt pairs with XSmoke.kt; each Java package pairs with
    // <package_with_underscores>Smoke.kt (android.view -> android_viewSmoke.kt).
    @Test
    fun everyStubHasASmokeFile() {
        val expected = kotlinStubs().map { it.nameWithoutExtension + "Smoke.kt" } +
            javaStubs().mapNotNull { packageOf(it) }.distinct().map { it.replace('.', '_') + "Smoke.kt" }
        val missing = expected.distinct().filterNot { smokeDir.resolve(it).isFile }
        if (missing.isNotEmpty()) fail("stubs without a smoke file in ${smokeDir.path}: $missing")
    }

    // Java stubs must sit at the directory matching their package so K2 finds
    // them (javac above enforces one public top-level type named after the file).
    @Test
    fun javaStubsMatchTheirPackageDirectory() {
        val misplaced = javaStubs().mapNotNull { file ->
            val pkg = packageOf(file) ?: return@mapNotNull "${file.path} (no package)"
            val expectedDir = javaStubsDir.resolve(pkg.replace('.', File.separatorChar))
            if (file.parentFile.canonicalFile != expectedDir.canonicalFile) "${file.path} (package $pkg)" else null
        }
        if (misplaced.isNotEmpty()) fail("Java stubs outside their package directory: $misplaced")
    }

    // A stub in a package the JDK already provides shadows the real classes.
    @Test
    fun noStubDeclaresAJdkPackage() {
        val jdkPackages = ModuleLayer.boot().modules().flatMap { it.packages }.toSet()
        val shadowing = (kotlinStubs() + javaStubs()).mapNotNull { file ->
            val pkg = packageOf(file) ?: return@mapNotNull null
            if (pkg in jdkPackages) "${file.name} (package $pkg)" else null
        }
        if (shadowing.isNotEmpty()) fail("stubs must not declare JDK packages: $shadowing")
    }
}
