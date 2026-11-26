package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import java.io.File
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.jvm.K2JVMCompiler
import org.jetbrains.kotlin.config.Services
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class TestWithoutAssertionFilesTest {
    @Test fun reviewerExamplesMatchTheirGolden() {
        object : AbstractDiagnosticTest() {}.runDiagnosticTest("testingquality/TestWithoutAssertionReviewer.kt")
    }

    @Test fun goExemptionGoldensCompileCleanly() {
        for (name in listOf(
                "GoExemptions", "BenchmarkWildcard", "ComposeBenchmarkWildcard", "RoborazziWildcard",
                "EspressoVisibility", "VisibilityLookalike",
            )) {
            val file = File("src/test/data/diagnostic/testingquality/TestWithoutAssertion$name.kt")
            val source = file.readText().replace(Regex("<![A-Za-z][A-Za-z0-9_]*!>|<!>"), "")
            val result = compile(mapOf(file.name to source))
            assertTrue(result.clean, "${file.name}: ${result.problems()}")
        }
    }

    private fun compile(
        sources: Map<String, String>,
        options: Map<String, Any?> = emptyMap(),
        scanPaths: Map<String, String> = emptyMap(),
    ) = KritFirProbe.compile(
        sources,
        FirRuleCompileContext(
            enabledRuleIds = setOf("TestWithoutAssertion"),
            ruleConfigs = mapOf("TestWithoutAssertion" to options),
        ),
        scanPaths = scanPaths,
    )

    private fun findings(sources: Map<String, String>, options: Map<String, Any?> = emptyMap()): List<KritFirProbe.Diag> {
        val result = compile(sources, options)
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "TestWithoutAssertion" }
    }

    // A helper in another file has no body FIR may read, so it proves nothing either way and Go's finding stands for
    // both tests: FIR reports them, and the confirm-only merge keeps Go's verdict (#1671).
    @Test fun crossFileHelpersAreNoProof() {
        val helpers = """
            package support
            import org.junit.Assert.assertEquals
            class Asserting {
                fun consumeReady() { hop() }
                private fun hop() { assertEquals(1, 1) }
            }
            class Plain {
                fun consumeReady() { println("work") }
            }
        """.trimIndent()
        val tests = """
            import org.junit.Test
            import support.Asserting
            import support.Plain
            class HelpersTest {
                @Test fun asserted() { Asserting().consumeReady() }
                @Test fun unverified() { Plain().consumeReady() }
            }
        """.trimIndent()
        assertEquals(listOf(5, 6), findings(mapOf("Helpers.kt" to helpers, "HelpersTest.kt" to tests)).map { it.line })
    }

    // #1671: constructor calls, property reads, assignments in a helper, and helpers in another file prove no
    // assertion. TestWithoutAssertion is confirm-only, so FIR reports these and Go's findings stand.
    @Test fun ordinaryCallsAndReadsAreNotAssertions() {
        val library = """
            package demo.lib
            fun greet(name: String): String = "hi ${'$'}name"
            class Initializer(private val names: List<String>) {
                fun start() { names.forEach { greet(it) } }
            }
        """.trimIndent()
        val other = """
            package demo
            class Holder { var value: Int = 0 }
            fun bump(holder: Holder): Int {
                holder.value += 1
                return holder.value
            }
        """.trimIndent()
        val tests = """
            package demo
            import demo.lib.Initializer
            import demo.lib.greet
            import org.junit.Test
            class Local(private val n: Int) { fun run(): Int = n + 1 }
            object Config { val name: String = "x" }
            class NoAssertTest {
                private val local = Local(1)
                @Test fun callsLocalFunction() { bump(Holder()) }
                @Test fun constructsAndCalls() { Local(1).run() }
                @Test fun readsProperty() { local.run() }
                @Test fun readsObjectProperty() { println(Config.name) }
                @Test fun callsOtherModule() { Initializer(listOf("a")).start() }
                @Test fun callsOtherModuleTopLevel() { greet("a") }
                @Test fun assignsLocally() { var x = 0; x = Config.name.length; println(x) }
            }
        """.trimIndent()
        val sources = mapOf("Greet.kt" to library, "Other.kt" to other, "NoAssertTest.kt" to tests)
        assertEquals((9..15).toList(), findings(sources).filter { it.file == "NoAssertTest.kt" }.map { it.line })
    }

    @Test fun binaryModuleAndFriendSourceSetHelpersAreNoProof() {
        val directory = kotlin.io.path.createTempDirectory("krit-test-helpers").toFile()
        try {
            val source = directory.resolve("Library.kt")
            source.writeText("package dependency\nfun operate() { println(1) }\nfun condition() { require(true) }\n")
            val output = directory.resolve("classes")
            val exit = K2JVMCompiler().exec(MessageCollector.NONE, Services.EMPTY, K2JVMCompilerArguments().apply {
                freeArgs = listOf(source.absolutePath)
                destination = output.absolutePath
                moduleName = "testFixtures"
                noStdlib = true
                noReflect = true
                classpath = System.getProperty("kotlin.stdlib.jar")
            })
            assertEquals(ExitCode.OK, exit)
            for (friend in listOf(false, true)) {
                val result = KritFirProbe.compile(
                    mapOf("Checks.kt" to """
                        import org.junit.Test
                        import dependency.operate
                        import dependency.condition
                        class Checks {
                            @Test fun operation() { operate() }
                            @Test fun hiddenAssertion() { condition() }
                        }
                    """.trimIndent()),
                    FirRuleCompileContext(enabledRuleIds = setOf("TestWithoutAssertion")),
                ) { arguments ->
                    arguments.classpath = listOfNotNull(arguments.classpath, output.absolutePath).joinToString(File.pathSeparator)
                    if (friend) arguments.friendPaths = arrayOf(output.absolutePath)
                }
                assertTrue(result.clean, result.problems())
                assertEquals(listOf(5, 6), result.diags.filter { it.name == "TestWithoutAssertion" }.map { it.line })
            }
        } finally {
            directory.deleteRecursively()
        }
    }

    @Test fun goNameExemptionsAreNeverTurnedIntoFirFindings() {
        val source = """
            package ordinary
            import org.junit.Test
            fun assertEquals(expected: Int, actual: Int) { println(actual) }
            fun verify() { println("work") }
            class LookalikesTest {
                @Test fun equality() { assertEquals(1, 1) }
                @Test fun verification() { verify() }
            }
        """.trimIndent()
        assertEquals(emptyList(), findings(mapOf("LookalikesTest.kt" to source)))
    }

    // A cycle and a conditional throw in a helper still suppress the report; a stub that only throws does not.
    @Test fun cyclesAndConditionalThrowsSuppressButAStubDoesNot() {
        val source = """
            import org.junit.Test
            class HelpersTest {
                @Test fun cyclic() { first() }
                @Test fun checked() { condition(true) }
                @Test fun stubbed() { stub() }
                private fun first() { second() }
                private fun second() { first() }
                private fun condition(ready: Boolean) { if (!ready) throw IllegalStateException("not ready") }
                private fun stub() { throw NotImplementedError("later") }
            }
        """.trimIndent()
        assertEquals(listOf(5), findings(mapOf("HelpersTest.kt" to source)).map { it.line })
    }

    @Test fun optionsMatchResolvedDeclaredNamesNotImportAliases() {
        val source = """
            package ordinary
            import org.junit.Test
            import ordinary.customReady as perform
            class OptionsTest {
                @Test fun loads() { perform() }
            }
            fun customReady() { println("work") }
        """.trimIndent()
        val sources = mapOf("OptionsTest.kt" to source)
        assertEquals(listOf(5), findings(sources).map { it.line })
        assertEquals(emptyList(), findings(sources, mapOf("allowNoAssertionTests" to true)))
        assertEquals(emptyList(), findings(sources, mapOf("assertionMethodPatterns" to listOf("custom*"))))
        assertEquals(listOf(5), findings(sources, mapOf("assertionMethodPatterns" to listOf("another*"))).map { it.line })
    }

    @Test fun unresolvedCallsAndReceiverTypesAbstain() {
        val sources = listOf(
            "@org.junit.Test fun missing() { missingCall() }",
            "@org.junit.Test fun missing() { val receiver: MissingType = TODO(); receiver.run() }",
            "@org.junit.Test fun missing() { println(missingValue) }",
        )
        for (source in sources) {
            val result = compile(mapOf("MissingTest.kt" to source))
            assertFalse(result.clean)
            assertEquals(emptyList(), result.diags.filter { it.name == "TestWithoutAssertion" })
        }
    }

    @Test fun kotlinTestAssertionsUseTheRealLibrary() {
        val jar = requireNotNull(System.getProperty("kotlin.test.jar"))
        val stdlib = System.getProperty("kotlin.stdlib.jar")
        val source = """
            import org.junit.Test
            import kotlin.test.assertEquals as compareValues
            import kotlin.test.assertFailsWith
            class KotlinAssertionsTest {
                @Test fun equality() { compareValues(1, 1) }
                @Test fun failure() { assertFailsWith<IllegalStateException> { println("work") } }
                @Test fun missing() { println("work") }
            }
        """.trimIndent()
        val result = KritFirProbe.compile(
            mapOf("KotlinAssertionsTest.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("TestWithoutAssertion")),
            configure = { it.classpath = listOfNotNull(stdlib, jar).joinToString(File.pathSeparator) },
        )
        assertTrue(result.clean, result.problems())
        assertEquals(listOf(7), result.diags.filter { it.name == "TestWithoutAssertion" }.map { it.line })
    }

    @Test fun testDslLambdasAloneAreNotTestDeclarations() {
        val source = """
            fun test(body: () -> Unit) { body() }
            fun scenario() { test { println("work") } }
        """.trimIndent()
        assertEquals(emptyList(), findings(mapOf("Dsl.kt" to source)))
    }

    @Test fun editorTemplateExemptionUsesTheScansPath() {
        val sources = mapOf("Template.kt" to "@org.junit.Test fun loads() { println(42) }")
        val editor = compile(sources, scanPaths = mapOf("Template.kt" to ".idea/fileTemplates/Test.kt"))
        assertTrue(editor.clean, editor.problems())
        assertEquals(emptyList(), editor.diags.filter { it.name == "TestWithoutAssertion" })
        val ordinary = compile(sources, scanPaths = mapOf("Template.kt" to "src/test/Template.kt"))
        assertTrue(ordinary.clean, ordinary.problems())
        assertEquals(listOf(1), ordinary.diags.filter { it.name == "TestWithoutAssertion" }.map { it.line })
    }

    @Test fun aDeepSourceHelperChainAbstainsInsteadOfGuessingBeyondTheBudget() {
        val source = """
            import org.junit.Test
            import org.junit.Assert.assertEquals
            class DeepTest {
                @Test fun deeplyChecked() { first() }
                private fun first() { second() }
                private fun second() { third() }
                private fun third() { fourth() }
                private fun fourth() { fifth() }
                private fun fifth() { assertEquals(1, 1) }
            }
        """.trimIndent()
        assertEquals(emptyList(), findings(mapOf("DeepTest.kt" to source)))
    }
}
