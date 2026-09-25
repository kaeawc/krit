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
import org.junit.jupiter.api.assertThrows
import org.opentest4j.AssertionFailedError
import org.opentest4j.TestAbortedException
import java.io.File
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import kotlin.test.fail

// Fixture parity, the fast lane-local tier: every built-in FIR rule must agree
// with the Go rule's own fixtures. For each FirRule discovered on the
// classpath, the Go fixtures tests/fixtures/{positive,negative}/<category>/<RuleId>.kt
// are compiled against the stub library with only that rule enabled:
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
// A marker on a fixture that compiles cleanly is stale and fails. A rule with a
// FIR checker but no rule entry in schemas/krit-config.schema.json, or without
// both .kt fixtures, fails. The exact per-line comparison against the Go rule
// is the second tier, TestFirFixtureParity in tests/parity. The authoring
// checklist lives in docs/fir-checker-authoring.md.
class FixtureParityTest {

    @TestFactory
    fun fixtureParity(): List<DynamicNode> {
        val repo = repoRoot()
        val known = schemaRuleIds(repo)
        val rules = parityRules()
        return rules.map { rule -> ruleContainer(rule, repo, known) } +
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
        ).also { assertTrue(it.clean, it.problems()) }
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

    // The schema lists every registered Go rule, including the Android-lint
    // family that registers its IDs positionally rather than via RuleName.
    @Test
    fun schemaListsRegisteredRules() {
        val ids = schemaRuleIds(repoRoot())
        val expected = setOf(
            "DefaultLocale", "CommitPrefEdits",
            "InjectDispatcher", "ComposeRememberWithoutKey", "CollectInOnCreateWithoutLifecycle",
        )
        assertTrue(ids.containsAll(expected), "schema is missing ${expected - ids}")
        assertTrue("UnsafeCastWhenNullable" !in ids, "UnsafeCastWhenNullable has no Go rule")
        assertTrue("android-lint" !in ids && "config" !in ids, "rule-set names leaked into rule IDs")
    }

    // A skip marker must not outlive the reason for it: once the fixture
    // compiles cleanly, the marker is stale and fails.
    @Test
    fun staleSkipMarkerFails() {
        val clean = "// fir-parity: skip needed a missing stub\npackage stale\nfun f() {}\n"
        val error = assertThrows<AssertionFailedError> {
            checkFixture("InjectDispatcher", "synthetic/Stale.kt", "Stale.kt", clean, positive = false)
        }
        assertTrue("stale skip marker" in error.message.orEmpty(), error.message)
    }

    @Test
    fun skipMarkerOnNonCompilingFixtureSkips() {
        val broken = "// fir-parity: skip uses an unstubbed library\npackage broken\nfun f() { missingCall() }\n"
        val aborted = assertThrows<TestAbortedException> {
            checkFixture("InjectDispatcher", "synthetic/Broken.kt", "Broken.kt", broken, positive = false)
        }
        assertTrue("uses an unstubbed library" in aborted.message.orEmpty(), aborted.message)
    }

    @Test
    fun nonCompilingFixtureWithoutMarkerFails() {
        val broken = "package broken2\nfun f() { missingCall() }\n"
        val error = assertThrows<AssertionFailedError> {
            checkFixture("InjectDispatcher", "synthetic/Broken2.kt", "Broken2.kt", broken, positive = false)
        }
        assertTrue("Unresolved symbols: missingCall" in error.message.orEmpty(), error.message)
    }

    private fun ruleContainer(rule: FirRule, repo: File, known: Set<String>): DynamicContainer {
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
            add(parityTest(id, "registered as a Go rule") {
                if (id !in known) {
                    fail(
                        "FIR rule '$id' (${rule::class.java.name}) is not a Go rule: schemas/krit-config.schema.json " +
                            "has no entry for it. krit only sends krit-fir the IDs of active Go rules, so this checker " +
                            "never runs; ruleId must equal the Go rule ID exactly. If the Go rule is new, run `make schema`.",
                    )
                }
            })
            if (positives.isEmpty() || negatives.isEmpty()) {
                add(parityTest(id, "has Go fixtures") {
                    val missing = listOfNotNull(
                        "tests/fixtures/positive/<category>/$id.kt".takeIf { positives.isEmpty() },
                        "tests/fixtures/negative/<category>/$id.kt".takeIf { negatives.isEmpty() },
                    )
                    fail(
                        "FIR rule '$id' has no Go fixture(s): $missing. Every ported rule needs both, as Kotlin " +
                            "(.kt) files: FIR checks Kotlin, so a .java-only fixture does not count.",
                    )
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
        // A unique file name per fixture: the probe reports by file name.
        val name = "${kind}_${fixture.parentFile.name}_${fixture.name}".replace('-', '_')
        return parityTest(ruleId, "$kind $rel") {
            checkFixture(ruleId, rel, name, fixture.readText(), positive)
        }
    }

    // Compiles one fixture with only [ruleId] enabled and asserts its verdict.
    // The compile runs first even for an opted-out fixture, so a marker whose
    // reason no longer holds (the fixture now compiles) is caught.
    private fun checkFixture(ruleId: String, rel: String, name: String, source: String, positive: Boolean) {
        val skip = skipReason(source, rel)
        val result = KritFirProbe.compile(
            mapOf(name to source),
            FirRuleCompileContext(enabledRuleIds = setOf(ruleId)),
        )
        if (skip != null) {
            if (result.clean) {
                fail(
                    "$rel carries `// fir-parity: skip $skip` but compiles cleanly against the stubs: stale skip " +
                        "marker. Remove it so the fixture is checked.",
                )
            }
            Assumptions.abort<Unit>("$rel opted out of FIR fixture parity: $skip")
        }
        if (result.crashed) fail("$ruleId crashed compiling $rel (K2 INTERNAL_ERROR); see the exception above.")
        if (!result.clean) fail(compileFailure(rel, result))
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

    // A dynamic test named "<RuleId>: <label>" whose outcome also lands in the
    // per-fixture summary printed after the class.
    private fun parityTest(ruleId: String, label: String, body: () -> Unit): DynamicTest =
        DynamicTest.dynamicTest("$ruleId: $label") {
            val name = "$ruleId: $label"
            try {
                body()
            } catch (e: TestAbortedException) {
                summary += name to "SKIP"
                throw e
            } catch (e: Throwable) {
                summary += name to "FAIL"
                throw e
            }
            summary += name to "PASS"
        }

    private fun compileFailure(rel: String, result: KritFirProbe.Compilation): String = buildString {
        val errors = result.compileErrors + result.otherErrors
        val unresolved = errors.flatMap { e -> unresolvedRe.findAll(e).map { it.groupValues[1] } }.distinct()
        appendLine("$rel does not compile cleanly against the stub library, so its FIR verdict would be vacuous.")
        if (unresolved.isNotEmpty()) appendLine("Unresolved symbols: ${unresolved.joinToString(", ")}")
        appendLine("Compiler errors:")
        appendLine(result.problems().prependIndent("  "))
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
        // tests/parity/fir_parity_test.go keeps the same list.
        private val FIR_ONLY_RULES = mapOf(
            "UnsafeCastWhenNullable" to "no Go rule or Go fixtures (pre-harness FIR-only checker)",
        )

        // Probe rules from the main module's tests live here; they are protocol
        // fixtures, not ports of Go rules.
        private val TEST_ONLY_PACKAGES = listOf("dev.jasonpearson.krit.fir.checkers.protocol.")
        private val skipRe = Regex("""^\s*//\s*fir-parity:\s*skip\b(.*)$""", RegexOption.MULTILINE)
        private val unresolvedRe = Regex("""Unresolved reference '([^']+)'""")

        // Rule IDs from the generated config schema (`make schema`), which has
        // one entry per registered Go rule: properties.<ruleSet>.properties.<RuleId>,
        // recognized by its `active` option.
        private fun schemaRuleIds(repo: File): Set<String> {
            val file = repo.resolve("schemas/krit-config.schema.json")
            val root = JsonReader(file.readText()).value() as Map<*, *>
            val ruleSets = root["properties"] as? Map<*, *> ?: error("${file.path}: no top-level properties")
            return ruleSets.values.flatMap { ruleSet ->
                val rules = (ruleSet as? Map<*, *>)?.get("properties") as? Map<*, *> ?: return@flatMap emptyList()
                rules.mapNotNull { (id, rule) ->
                    val options = (rule as? Map<*, *>)?.get("properties") as? Map<*, *>
                    (id as String).takeIf { options?.containsKey("active") == true }
                }
            }.toSet()
        }

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

    // Minimal JSON reader for the schema (the test classpath has no JSON
    // library): objects, arrays, strings, and scalars kept as raw text.
    private class JsonReader(private val text: String) {
        private var pos = 0

        fun value(): Any? {
            space()
            return when (text[pos]) {
                '{' -> obj()
                '[' -> array()
                '"' -> string()
                else -> scalar()
            }
        }

        private fun obj(): Map<String, Any?> {
            pos++
            val out = linkedMapOf<String, Any?>()
            space()
            if (text[pos] == '}') return out.also { pos++ }
            while (true) {
                space()
                val key = string()
                space()
                require(text[pos++] == ':') { "malformed JSON object at $pos" }
                out[key] = value()
                space()
                if (text[pos++] == '}') return out
            }
        }

        private fun array(): List<Any?> {
            pos++
            val out = mutableListOf<Any?>()
            space()
            if (text[pos] == ']') return out.also { pos++ }
            while (true) {
                out += value()
                space()
                if (text[pos++] == ']') return out
            }
        }

        private fun string(): String {
            require(text[pos++] == '"') { "expected a JSON string at $pos" }
            val out = StringBuilder()
            while (true) {
                val c = text[pos++]
                when (c) {
                    '"' -> return out.toString()
                    '\\' -> {
                        val e = text[pos++]
                        if (e == 'u') {
                            out.append(text.substring(pos, pos + 4).toInt(16).toChar())
                            pos += 4
                        } else {
                            out.append(mapOf('n' to '\n', 't' to '\t', 'r' to '\r', 'b' to '\b', 'f' to '\u000c')[e] ?: e)
                        }
                    }
                    else -> out.append(c)
                }
            }
        }

        private fun scalar(): String {
            val start = pos
            while (pos < text.length && text[pos] !in ",]} \t\r\n") pos++
            return text.substring(start, pos)
        }

        private fun space() {
            while (pos < text.length && text[pos].isWhitespace()) pos++
        }
    }
}
