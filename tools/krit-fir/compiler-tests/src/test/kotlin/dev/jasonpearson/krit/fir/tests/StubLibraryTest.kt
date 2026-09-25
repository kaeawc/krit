package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import java.io.File
import kotlin.test.assertTrue
import kotlin.test.fail

// Structural guards for the shared source stubs in src/test/data/stubs/.
class StubLibraryTest {

    private val stubsDir = File("src/test/data/stubs")
    private val smokeDir = File("src/test/data/diagnostic/stubs")

    private fun stubFiles(): List<File> =
        stubsDir.listFiles { f -> f.extension == "kt" }.orEmpty().sortedBy { it.name }

    // KritFirProbe only reports compiler errors located in *requested* sources,
    // so an error inside a stub (a bad override, a redeclaration) would stay
    // invisible unless some smoke file happened to trip over it. Requesting
    // every stub under its own file name makes the probe treat each stub as a
    // requested source: any stub error fails here, and no krit rule may fire on
    // a stub declaration.
    @Test
    fun stubsCompileCleanly() {
        val stubs = stubFiles()
        assertTrue(stubs.isNotEmpty(), "no stubs found in ${stubsDir.path}")
        val diags = KritFirProbe.diagnose(stubs.associate { it.name to it.readText() })
        if (diags.isNotEmpty()) fail("krit rules fired on stub declarations:\n" + diags.joinToString("\n"))
    }

    @Test
    fun everyStubHasASmokeFile() {
        val missing = stubFiles()
            .map { it.nameWithoutExtension + "Smoke.kt" }
            .filterNot { smokeDir.resolve(it).isFile }
        if (missing.isNotEmpty()) fail("stub files without a smoke file in ${smokeDir.path}: $missing")
    }

    // A stub in a package the JDK already provides shadows the real classes.
    @Test
    fun noStubDeclaresAJdkPackage() {
        val jdkPackages = ModuleLayer.boot().modules().flatMap { it.packages }.toSet()
        val packageRe = Regex("""^package\s+([\w.]+)""", RegexOption.MULTILINE)
        val shadowing = stubFiles().mapNotNull { file ->
            val pkg = packageRe.find(file.readText())?.groupValues?.get(1) ?: return@mapNotNull null
            if (pkg in jdkPackages) "${file.name} (package $pkg)" else null
        }
        if (shadowing.isNotEmpty()) fail("stubs must not declare JDK packages: $shadowing")
    }
}
