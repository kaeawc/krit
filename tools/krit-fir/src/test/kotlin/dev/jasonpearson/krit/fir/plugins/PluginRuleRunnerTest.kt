package dev.jasonpearson.krit.fir.plugins

import dev.jasonpearson.krit.api.KritFile
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

/**
 * End-to-end coverage for the analyzeFile RPC: a real plugin JAR is
 * compiled at test time, loaded into [PluginRuleRegistry], and
 * executed against a synthetic Kotlin source. We verify the wire
 * shape that ships back through [PluginResponse.buildAnalyzeFile]
 * mirrors krit-types' `buildAnalyzeFileResponse`.
 */
class PluginRuleRunnerTest {

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
    fun lineScannerRuleProducesFindingsWithoutPsiOrResolver() {
        // The rule below scans the file text for "FORBIDDEN" and
        // emits one finding per match. It doesn't need PSI or a
        // Resolver — i.e. the exact rule shape PR 3.2 targets.
        val jar = buildLineScannerJar("ForbiddenWordRule")
        PluginRuleRegistry.load(listOf(jar.absolutePath))

        val file = KritFile(
            path = "/tmp/Sample.kt",
            text = "val a = 1\nval b = FORBIDDEN\n",
            ktFile = null,
        )
        val result = PluginRuleRunner.run(file, ruleIds = null, ruleConfigs = null)
        assertEquals(emptyMap(), result.errors)
        assertEquals(1, result.findings.size, "expected one match, got ${result.findings}")
        val finding = result.findings.single()
        assertEquals("ForbiddenWordRule", finding.ruleId)
        assertEquals("/tmp/Sample.kt", finding.file)
        assertEquals(2, finding.line)
        assertEquals("warning", finding.severity)
    }

    @Test
    fun ruleThrowsBubblesAsErrorEntryNotCrash() {
        // Per-rule exception handling must isolate failures so one
        // broken rule doesn't kill the entire analyzeFile request.
        // Mirrors krit-types' behavior.
        val jar = buildThrowingRuleJar("BrokenRule")
        PluginRuleRegistry.load(listOf(jar.absolutePath))

        val file = KritFile(path = "/tmp/Any.kt", text = "fun x() {}", ktFile = null)
        val result = PluginRuleRunner.run(file, ruleIds = null, ruleConfigs = null)
        assertEquals(emptyList(), result.findings)
        assertEquals(setOf("BrokenRule"), result.errors.keys)
        assertTrue("boom" in (result.errors["BrokenRule"] ?: ""), result.errors.toString())
    }

    @Test
    fun selectedRuleIdsFilterToOnlyTheRequestedSubset() {
        val one = buildLineScannerJar("OneRule")
        val two = buildLineScannerJar("TwoRule")
        PluginRuleRegistry.load(listOf(one.absolutePath, two.absolutePath))

        val file = KritFile(path = "/tmp/Both.kt", text = "FORBIDDEN line\n", ktFile = null)
        val result = PluginRuleRunner.run(file, ruleIds = listOf("TwoRule"), ruleConfigs = null)
        assertEquals(setOf("TwoRule"), result.findings.map { it.ruleId }.toSet())
    }

    @Test
    fun ruleConfigsArePropagatedAsConfigOptions() {
        // The rule reads `keyword` from RuleContext.config and emits
        // a finding per line matching it. This confirms the
        // ruleConfigs map plumbs through end-to-end.
        val jar = buildKeywordRuleJar("KeywordRule")
        PluginRuleRegistry.load(listOf(jar.absolutePath))

        val file = KritFile(
            path = "/tmp/Configured.kt",
            text = "println(\"hi\")\nprintln(\"BYE\")\n",
            ktFile = null,
        )
        val result = PluginRuleRunner.run(
            file = file,
            ruleIds = null,
            ruleConfigs = mapOf("KeywordRule" to mapOf("keyword" to "BYE")),
        )
        assertEquals(1, result.findings.size, "expected one match for 'BYE': ${result.findings}")
        assertEquals(2, result.findings.single().line)
    }

    @Test
    fun gradleContextOnlyVisibleToRulesThatDeclareNeedsGradle() {
        // A rule that declared NEEDS_GRADLE sees a non-null context.
        val needsJar = buildGradleAwareRuleJar("GradleAwareRule", declareNeed = true)
        PluginRuleRegistry.load(listOf(needsJar.absolutePath))
        val payloads = ProjectPayloads(
            gradle = GradleProfilePayload(
                minSdk = 24, targetSdk = 34, compileSdk = 34,
                kotlinVersion = "2.3.21", javaTargetVersion = "21", agpVersion = "8.5.0",
                deps = listOf("org.x:y:1.0"),
            ),
            manifest = null, resources = null, moduleIndex = null, crossFile = null,
        )
        val file = KritFile(path = "/tmp/G.kt", text = "fun x() {}", ktFile = null)
        val withGate = PluginRuleRunner.run(
            file = file, ruleIds = null, ruleConfigs = null,
            projectPayloads = payloads,
        )
        assertEquals(1, withGate.findings.size, "expected gradle-aware rule to fire: ${withGate.findings}")
        assertEquals("gradle:24", withGate.findings.single().message)

        // Same rule but without the capability declaration — even
        // though we pass the payload, the rule shouldn't see it
        // (otherwise the capability gate is meaningless).
        PluginRuleRegistry.resetForTesting()
        val noNeedsJar = buildGradleAwareRuleJar("UndeclaredGradleRule", declareNeed = false)
        PluginRuleRegistry.load(listOf(noNeedsJar.absolutePath))
        val withoutGate = PluginRuleRunner.run(
            file = file, ruleIds = null, ruleConfigs = null,
            projectPayloads = payloads,
        )
        assertEquals(emptyList(), withoutGate.findings, "rule without NEEDS_GRADLE must see gradle=null")
    }

    @Test
    fun analyzeFileResponseShapeMatchesKritTypes() {
        val findings = listOf(
            PluginRuleRunner.PluginFinding(
                file = "/tmp/F.kt",
                line = 3,
                column = 5,
                startByte = 12,
                endByte = 20,
                ruleSet = "custom",
                ruleId = "R",
                severity = "warning",
                message = "msg",
                confidence = 0.9,
                fix = null,
            ),
        )
        val response = PluginResponse.buildAnalyzeFile(id = 11, findings = findings, errors = emptyMap())
        assertTrue(response.startsWith("""{"id":11,"result":{"findings":["""), response)
        assertTrue(""""file":"/tmp/F.kt"""" in response, response)
        assertTrue(""""line":3""" in response, response)
        assertTrue(""""column":5""" in response, response)
        assertTrue(""""confidence":0.9""" in response, response)
        assertTrue("fix" !in response, response)
        // No errors → no errors field. Matches krit-types' omit-when-empty rule.
        assertTrue(""""errors":""" !in response, response)
    }

    @Test
    fun analyzeFileResponseIncludesFixAndErrorsWhenPresent() {
        val response = PluginResponse.buildAnalyzeFile(
            id = 7,
            findings = listOf(
                PluginRuleRunner.PluginFinding(
                    file = "/F.kt",
                    line = 1, column = 1, startByte = 0, endByte = 0,
                    ruleSet = "custom", ruleId = "R", severity = "warning",
                    message = "m", confidence = 0.5,
                    fix = PluginRuleRunner.PluginFix(
                        startLine = 1, endLine = 1, replacement = "rep", safety = "idiomatic",
                    ),
                ),
            ),
            errors = mapOf("BadRule" to "broken"),
        )
        assertTrue(""""fix":{"startLine":1,"endLine":1,"replacement":"rep","safety":"idiomatic"}""" in response, response)
        assertTrue(""""errors":{"BadRule":"broken"}""" in response, response)
    }

    // ── Test plumbing ────────────────────────────────────────────────

    private fun buildLineScannerJar(ruleId: String): File {
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
                needs = [],
            )
            class $ruleId : KritRule {
                override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
                    val out = mutableListOf<Finding>()
                    file.text.split('\n').forEachIndexed { i, line ->
                        if ("FORBIDDEN" in line) {
                            out += Finding(message = "forbidden", line = i + 1)
                        }
                    }
                    return out
                }
            }
        """.trimIndent()
        return buildJar(ruleId, source)
    }

    private fun buildThrowingRuleJar(ruleId: String): File {
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
                needs = [],
            )
            class $ruleId : KritRule {
                override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
                    throw IllegalStateException("boom")
                }
            }
        """.trimIndent()
        return buildJar(ruleId, source)
    }

    private fun buildGradleAwareRuleJar(ruleId: String, declareNeed: Boolean): File {
        val needsExpr = if (declareNeed) "dev.jasonpearson.krit.api.Capability.NEEDS_GRADLE" else ""
        val source = """
            package dev.jasonpearson.krit.fir.plugins.testrules

            import dev.jasonpearson.krit.api.Capability
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
                override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
                    val sdk = ctx.gradle?.minSdk ?: return emptyList()
                    return listOf(Finding(message = "gradle:" + sdk, line = 1))
                }
            }
        """.trimIndent()
        return buildJar(ruleId, source)
    }

    private fun buildKeywordRuleJar(ruleId: String): File {
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
                needs = [],
            )
            class $ruleId : KritRule {
                override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
                    val keyword = ctx.stringOption("keyword")
                    if (keyword.isEmpty()) return emptyList()
                    val out = mutableListOf<Finding>()
                    file.text.split('\n').forEachIndexed { i, line ->
                        if (keyword in line) out += Finding(message = "match", line = i + 1)
                    }
                    return out
                }
            }
        """.trimIndent()
        return buildJar(ruleId, source)
    }

    private fun buildJar(ruleId: String, source: String): File {
        val srcDir = tmp.resolve("gen-${ruleId}-src").toFile().apply { mkdirs() }
        val outDir = tmp.resolve("gen-${ruleId}-out").toFile().apply { mkdirs() }
        val sourceFile = srcDir.resolve("$ruleId.kt")
        sourceFile.writeText(source)

        val classpath = System.getProperty("java.class.path") ?: ""
        val compiler = org.jetbrains.kotlin.cli.jvm.K2JVMCompiler()
        val args = org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments().apply {
            freeArgs = listOf(sourceFile.absolutePath)
            destination = outDir.absolutePath
            this.classpath = classpath
            noStdlib = true
            noReflect = true
        }
        val exit = compiler.exec(
            org.jetbrains.kotlin.cli.common.messages.MessageCollector.NONE,
            org.jetbrains.kotlin.config.Services.EMPTY,
            args,
        )
        check(exit.code == 0) { "K2 compilation failed for $ruleId: $exit" }

        val jarFile = tmp.resolve("$ruleId.jar").toFile()
        val manifest = Manifest().apply {
            mainAttributes[Attributes.Name.MANIFEST_VERSION] = "1.0"
            mainAttributes.putValue("Krit-SDK-Version", SdkCompatibility.DAEMON_SDK_VERSION)
        }
        JarOutputStream(jarFile.outputStream(), manifest).use { jar ->
            jar.putNextEntry(ZipEntry("META-INF/services/dev.jasonpearson.krit.api.KritRule"))
            jar.write("dev.jasonpearson.krit.fir.plugins.testrules.$ruleId".toByteArray())
            jar.closeEntry()

            val classFile = outDir.resolve("dev/jasonpearson/krit/fir/plugins/testrules/$ruleId.class")
            assertNotNull(classFile.takeIf { it.isFile }, "compiled class missing: $classFile")
            jar.putNextEntry(ZipEntry("dev/jasonpearson/krit/fir/plugins/testrules/$ruleId.class"))
            jar.write(classFile.readBytes())
            jar.closeEntry()
        }
        return jarFile
    }
}
