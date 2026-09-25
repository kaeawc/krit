package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.FirRuleDiscovery
import org.junit.jupiter.api.AfterAll
import org.junit.jupiter.api.Assumptions
import org.junit.jupiter.api.DynamicContainer
import org.junit.jupiter.api.DynamicNode
import org.junit.jupiter.api.DynamicTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestFactory
import java.io.File
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import kotlin.test.fail

// Fixture parity: every built-in FIR rule must agree with the Go rule's own
// fixtures. For each FirRule discovered on the classpath, the Go fixtures
// tests/fixtures/{positive,negative}/<category>/<RuleId>.kt are compiled
// against the stub library with only that rule enabled:
//
//   positive -> at least one finding for the rule
//   negative -> no finding for the rule
//
// A fixture must compile cleanly against the stubs, otherwise the verdict is
// vacuous; extend the stubs (see src/test/data/stubs/README.md) or, when the
// fixture cannot be made to compile, opt it out with a marker line
//
//   // fir-parity: skip <reason>
//
// A rule with a FIR checker but no Go registry entry or no Go fixtures fails.
// The authoring checklist lives in docs/fir-checker-authoring.md.
class FixtureParityTest {

    @TestFactory
    fun fixtureParity(): List<DynamicNode> {
        val repo = repoRoot()
        val registered = goRegisteredRuleIds(repo)
        val rules = parityRules()
        return rules.map { rule -> ruleContainer(rule, repo, registered) } +
            staleExemptions(rules.map { it.ruleId }.toSet())
    }

    // Enabling one rule must disable the others inside the plugin; otherwise
    // "only that rule enabled" is not what the parity compile tests.
    @Test
    fun ruleContextSelectsTheEnabledRule() {
        val source = """
            package ctx
            import kotlinx.coroutines.Dispatchers
            import kotlinx.coroutines.withContext
            class C {
                suspend fun m() { withContext(Dispatchers.IO) { } }
            }
        """.trimIndent()
        fun count(enabled: String) = KritFirProbe.compile(
            mapOf("Ctx.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf(enabled)),
        ).also { assertTrue(it.clean, it.compileErrors.joinToString("\n")) }
            .diags.count { it.name == "InjectDispatcher" }
        assertTrue(count("InjectDispatcher") > 0, "InjectDispatcher enabled but silent")
        assertEquals(0, count("ComposeRememberWithoutKey"), "InjectDispatcher fired while disabled")
    }

    // Guards against the factory silently producing nothing (for example a
    // classpath change that hides the checkers from discovery).
    @Test
    fun discoversBuiltInCheckers() {
        val ids = parityRules().map { it.ruleId }
        assertTrue(ids.isNotEmpty(), "no built-in FIR rules discovered under dev.jasonpearson.krit.fir.checkers")
    }

    private fun ruleContainer(rule: FirRule, repo: File, registered: Set<String>): DynamicContainer {
        val id = rule.ruleId
        val exemption = FIR_ONLY_RULES[id]
        val positives = fixtures(repo, "positive", id)
        val negatives = fixtures(repo, "negative", id)
        val children = buildList<DynamicNode> {
            if (exemption != null) {
                add(parityTest(id, "exempt: $exemption") {
                    Assumptions.abort<Unit>("$id is exempt from fixture parity: $exemption")
                })
                return@buildList
            }
            add(parityTest(id, "registered in the Go registry") {
                if (id !in registered) {
                    fail(
                        "FIR rule '$id' (${rule::class.java.name}) has no Go registry entry " +
                            "(no `RuleName: \"$id\"` under internal/rules/). An ID the Go registry does not " +
                            "know is never enabled in production; ruleId must equal the Go rule ID exactly.",
                    )
                }
            })
            if (positives.isEmpty() || negatives.isEmpty()) {
                add(parityTest(id, "has Go fixtures") {
                    val missing = listOfNotNull(
                        "tests/fixtures/positive/<category>/$id.kt".takeIf { positives.isEmpty() },
                        "tests/fixtures/negative/<category>/$id.kt".takeIf { negatives.isEmpty() },
                    )
                    fail("FIR rule '$id' has no Go fixture(s): $missing. Every ported rule needs both.")
                })
            }
            positives.forEach { add(fixtureTest(id, repo, it, positive = true)) }
            negatives.forEach { add(fixtureTest(id, repo, it, positive = false)) }
        }
        return DynamicContainer.dynamicContainer(id, children)
    }

    private fun fixtureTest(ruleId: String, repo: File, fixture: File, positive: Boolean): DynamicTest {
        val rel = fixture.relativeTo(repo).invariantSeparatorsPath
        val kind = if (positive) "positive" else "negative"
        return parityTest(ruleId, "$kind $rel") {
            val source = fixture.readText()
            skipReason(source, rel)?.let { reason ->
                Assumptions.abort<Unit>("$rel opted out of FIR fixture parity: $reason")
            }
            // A unique file name per fixture: the probe reports by file name.
            val name = "${kind}_${fixture.parentFile.name}_${fixture.name}".replace('-', '_')
            val result = KritFirProbe.compile(
                mapOf(name to source),
                FirRuleCompileContext(enabledRuleIds = setOf(ruleId)),
            )
            if (result.crashed) fail("$ruleId crashed compiling $rel (K2 INTERNAL_ERROR); see the exception above.")
            if (result.compileErrors.isNotEmpty()) fail(compileFailure(rel, result.compileErrors))
            val hits = result.diags.filter { it.file == name && it.name == ruleId }
            if (positive && hits.isEmpty()) {
                fail("$ruleId: positive fixture $rel produced no FIR finding (the Go rule flags it) — false negative.")
            }
            if (!positive && hits.isNotEmpty()) {
                fail(
                    "$ruleId: negative fixture $rel produced ${hits.size} FIR finding(s) on line(s) " +
                        "${hits.map { it.line }} (the Go rule does not flag it) — false positive.",
                )
            }
        }
    }

    // A dynamic test named "<RuleId>: <label>" whose outcome also lands in the
    // per-fixture summary printed after the class.
    private fun parityTest(ruleId: String, label: String, body: () -> Unit): DynamicTest =
        DynamicTest.dynamicTest("$ruleId: $label") {
            val name = "$ruleId: $label"
            try {
                body()
            } catch (e: org.opentest4j.TestAbortedException) {
                summary += name to "SKIP"
                throw e
            } catch (e: Throwable) {
                summary += name to "FAIL"
                throw e
            }
            summary += name to "PASS"
        }

    private fun compileFailure(rel: String, errors: List<String>): String = buildString {
        val unresolved = errors.flatMap { e -> unresolvedRe.findAll(e).map { it.groupValues[1] } }.distinct()
        appendLine("$rel does not compile cleanly against the stub library, so its FIR verdict would be vacuous.")
        if (unresolved.isNotEmpty()) appendLine("Unresolved symbols: ${unresolved.joinToString(", ")}")
        appendLine("Compiler errors:")
        errors.forEach { appendLine("  $it") }
        appendLine(
            "Extend the stubs (tools/krit-fir/compiler-tests/src/test/data/stubs/README.md: real signatures, " +
                "one declaration per line, a smoke file per stub) for library/platform symbols. If the fixture " +
                "cannot compile for another reason, add `// fir-parity: skip <reason>` to it.",
        )
    }

    // An exemption for a rule that no longer exists is stale.
    private fun staleExemptions(discovered: Set<String>): List<DynamicNode> =
        FIR_ONLY_RULES.keys.filter { it !in discovered }.map { id ->
            parityTest(id, "stale exemption") {
                fail("FIR_ONLY_RULES lists '$id', which has no FIR checker; remove the entry.")
            }
        }

    private fun parityRules(): List<FirRule> =
        FirRuleDiscovery.rules.filterNot { rule ->
            TEST_ONLY_PACKAGES.any { rule::class.java.name.startsWith(it) }
        }

    private fun fixtures(repo: File, kind: String, ruleId: String): List<File> =
        repo.resolve("tests/fixtures/$kind").listFiles { f -> f.isDirectory }.orEmpty()
            .map { it.resolve("$ruleId.kt") }
            .filter { it.isFile }
            .sortedBy { it.path }

    // Every `RuleName: "<id>"` registration under internal/rules (tests excluded).
    private fun goRegisteredRuleIds(repo: File): Set<String> =
        repo.resolve("internal/rules").walkTopDown()
            .filter { it.isFile && it.extension == "go" && !it.name.endsWith("_test.go") }
            .flatMap { f -> goRuleNameRe.findAll(f.readText()).map { it.groupValues[1] } }
            .toSet()

    private fun skipReason(source: String, rel: String): String? {
        val match = skipRe.find(source) ?: return null
        val reason = match.groupValues[1].trim()
        if (reason.isEmpty()) fail("$rel: `// fir-parity: skip` needs a reason, e.g. `// fir-parity: skip <why>`.")
        return reason
    }

    companion object {
        private val summary = java.util.concurrent.ConcurrentLinkedQueue<Pair<String, String>>()

        @JvmStatic
        @AfterAll
        fun printSummary() {
            if (summary.isEmpty()) return
            println("FIR fixture parity:")
            summary.sortedBy { it.first }.forEach { (name, status) -> println("  $status  $name") }
        }

        // Rules with a FIR checker but no Go rule. They are never enabled in
        // production, so there is nothing to be at parity with. Do not add to
        // this map: a new checker ports an existing Go rule and must pass parity.
        private val FIR_ONLY_RULES = mapOf(
            "UnsafeCastWhenNullable" to "no Go registry entry or Go fixtures (pre-harness FIR-only checker)",
        )

        // Probe rules from the main module's tests live here; they are protocol
        // fixtures, not ports of Go rules.
        private val TEST_ONLY_PACKAGES = listOf("dev.jasonpearson.krit.fir.checkers.protocol.")
        private val skipRe = Regex("""^\s*//\s*fir-parity:\s*skip\b(.*)$""", RegexOption.MULTILINE)
        private val unresolvedRe = Regex("""Unresolved reference '([^']+)'""")
        private val goRuleNameRe = Regex("""\bRuleName:\s*"([A-Za-z0-9_]+)"""")

        // The repo root is two levels above the krit-fir Gradle root; the build
        // passes it as krit.repo.root. Fall back to walking up from the working
        // directory (the compiler-tests project dir under Gradle).
        private fun repoRoot(): File {
            System.getProperty("krit.repo.root")?.let { prop ->
                val dir = File(prop).canonicalFile
                require(isRepoRoot(dir)) { "krit.repo.root=$prop is not the krit repo root (no go.mod + tests/fixtures)" }
                return dir
            }
            return generateSequence(File("").absoluteFile) { it.parentFile }.firstOrNull(::isRepoRoot)
                ?: error("krit repo root not found above ${File("").absolutePath}; set -Dkrit.repo.root")
        }

        private fun isRepoRoot(dir: File) =
            dir.resolve("go.mod").isFile && dir.resolve("tests/fixtures/positive").isDirectory
    }
}
