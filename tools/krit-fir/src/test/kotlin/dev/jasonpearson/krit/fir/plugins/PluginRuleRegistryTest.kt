package dev.jasonpearson.krit.fir.plugins

import dev.jasonpearson.krit.api.Capability
import dev.jasonpearson.krit.api.KritRule
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import java.util.jar.Attributes
import java.util.jar.JarOutputStream
import java.util.jar.Manifest
import java.util.zip.ZipEntry
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

class PluginRuleRegistryTest {

    @TempDir
    lateinit var tmp: Path

    @BeforeEach
    fun resetRegistryBefore() {
        PluginRuleRegistry.resetForTesting()
    }

    @AfterEach
    fun resetRegistryAfter() {
        PluginRuleRegistry.resetForTesting()
    }

    @Test
    fun loadIsIdempotentAcrossRepeatedCalls() {
        // The Go-side daemon calls listPlugins repeatedly. Each call
        // passes the same plugin jar paths; the second load() must
        // be a no-op rather than re-registering the rules.
        val jar = buildSampleJar("NoOpRule", needs = emptyList())
        PluginRuleRegistry.load(listOf(jar.absolutePath))
        val firstCount = PluginRuleRegistry.descriptors().size
        PluginRuleRegistry.load(listOf(jar.absolutePath))
        val secondCount = PluginRuleRegistry.descriptors().size
        assertEquals(firstCount, secondCount, "load() must be idempotent on the same jar path")
    }

    @Test
    fun rulesAreReturnedSortedByRuleId() {
        val jar = buildSampleJar("ZRule", "ARule")
        PluginRuleRegistry.load(listOf(jar.absolutePath))
        val descriptors = PluginRuleRegistry.descriptorsForJars(listOf(jar.absolutePath))
        assertEquals(listOf("ARule", "ZRule"), descriptors.map { it.ruleId })
    }

    // Note: end-to-end "unsupported capability triggers load error"
    // can't be expressed here because every `Capability` enum value
    // is SUPPORTED on the krit-fir backend. The capability gate's
    // classification logic and diagnostic-message format are covered
    // by [`PluginCapabilitiesTest`]; this e2e suite focuses on the
    // happy paths and the load-failure paths reachable through the
    // real API.

    @Test
    fun ruleDeclaringNeedsFirLoadsSuccessfullyOnFirBackend() {
        // The whole point of the krit-fir backend: NEEDS_FIR is
        // SUPPORTED here. krit-types would refuse the same jar.
        val jar = buildSampleJar(
            "FirRule",
            needs = listOf(Capability.NEEDS_FIR.name),
        )
        PluginRuleRegistry.load(listOf(jar.absolutePath))
        val descriptors = PluginRuleRegistry.descriptorsForJars(listOf(jar.absolutePath))
        assertEquals(listOf("FirRule"), descriptors.map { it.ruleId })
        assertEquals(emptyList(), PluginRuleRegistry.diagnosticsForJars(listOf(jar.absolutePath)))
    }

    @Test
    fun missingJarThrowsIllegalArgumentException() {
        val missing = tmp.resolve("nope.jar").toFile().absolutePath
        val error = runCatching { PluginRuleRegistry.load(listOf(missing)) }.exceptionOrNull()
        assertNotNull(error)
        assertTrue(error is IllegalArgumentException, "got ${error::class}: ${error.message}")
        assertTrue(error.message!!.contains("plugin jar not found"), error.message)
    }

    /**
     * Build a tiny `.jar` containing `NoOpRuleImpl`-style classes that
     * implement [KritRule] plus a `META-INF/services` entry that lists
     * them. Compiling actual Kotlin rules at test time would balloon
     * the suite — instead we hand-assemble the classfile bytes via
     * Java reflection through a service-loader friendly stub.
     */
    private fun buildSampleJar(
        vararg ruleIds: String,
        needs: List<String> = emptyList(),
    ): File {
        val jarFile = tmp.resolve("plugin-${ruleIds.joinToString("-")}.jar").toFile()
        val manifest = Manifest().apply {
            mainAttributes[Attributes.Name.MANIFEST_VERSION] = "1.0"
            mainAttributes.putValue("Krit-SDK-Version", SdkCompatibility.DAEMON_SDK_VERSION)
        }
        JarOutputStream(jarFile.outputStream(), manifest).use { jar ->
            // Service file pointing at each rule's stub class.
            val serviceEntry = ZipEntry("META-INF/services/${KritRule::class.java.name}")
            jar.putNextEntry(serviceEntry)
            val service = ruleIds.joinToString("\n") { "dev.jasonpearson.krit.fir.plugins.testrules.$it" }
            jar.write(service.toByteArray())
            jar.closeEntry()

            for (ruleId in ruleIds) {
                val classBytes = generateRuleClassWithK2(ruleId, needs)
                jar.putNextEntry(ZipEntry("dev/jasonpearson/krit/fir/plugins/testrules/$ruleId.class"))
                jar.write(classBytes)
                jar.closeEntry()
            }
        }
        return jarFile
    }

    /**
     * Compile a synthetic `KritRule` implementation through the K2
     * compiler that krit-fir already bundles. The class lives at
     * `dev.jasonpearson.krit.fir.plugins.testrules.<ruleId>`, carries
     * a `@KritRuleInfo` annotation with the requested needs list, and
     * its `check` returns no findings. Compiling at test time keeps
     * the rule jar wired to the same `krit-rule-api` the loader uses
     * — handwritten classfile bytes would drift the moment the API
     * changes.
     */
    private fun generateRuleClassWithK2(ruleId: String, needs: List<String>): ByteArray {
        val srcDir = tmp.resolve("gen-${ruleId}-src").toFile().apply { mkdirs() }
        val outDir = tmp.resolve("gen-${ruleId}-out").toFile().apply { mkdirs() }
        val needsExpr = needs.joinToString(", ") { "dev.jasonpearson.krit.api.Capability.$it" }
        val source = """
            package dev.jasonpearson.krit.fir.plugins.testrules

            import dev.jasonpearson.krit.api.Finding
            import dev.jasonpearson.krit.api.KritFile
            import dev.jasonpearson.krit.api.KritRule
            import dev.jasonpearson.krit.api.KritRuleInfo
            import dev.jasonpearson.krit.api.Language
            import dev.jasonpearson.krit.api.Maturity
            import dev.jasonpearson.krit.api.RuleContext
            import dev.jasonpearson.krit.api.Severity

            @KritRuleInfo(
                id = "$ruleId",
                category = "custom",
                severity = Severity.WARNING,
                maturity = Maturity.EXPERIMENTAL,
                languages = [Language.KOTLIN],
                needs = [$needsExpr],
            )
            class $ruleId : KritRule {
                override fun check(file: KritFile, ctx: RuleContext): List<Finding> = emptyList()
            }
        """.trimIndent()
        val sourceFile = srcDir.resolve("$ruleId.kt")
        sourceFile.writeText(source)

        val classpath = listOfNotNull(
            System.getProperty("java.class.path"),
        ).joinToString(File.pathSeparator)
        val compiler = org.jetbrains.kotlin.cli.jvm.K2JVMCompiler()
        val args = org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments().apply {
            freeArgs = listOf(sourceFile.absolutePath)
            destination = outDir.absolutePath
            this.classpath = classpath
            noStdlib = true
            noReflect = true
        }
        val exitCode = compiler.exec(
            org.jetbrains.kotlin.cli.common.messages.MessageCollector.NONE,
            org.jetbrains.kotlin.config.Services.EMPTY,
            args,
        )
        check(exitCode.code == 0) { "K2 compilation failed for $ruleId: $exitCode" }

        val produced = outDir
            .resolve("dev/jasonpearson/krit/fir/plugins/testrules/$ruleId.class")
        return produced.readBytes()
    }
}
