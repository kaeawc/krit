package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import java.util.Random

/**
 * Timing of planned checks against full checks per edit (#872). Not a regression test: it only runs under the
 * `incrementalBench` Gradle task, by hand, on an otherwise idle machine, and prints a table.
 *
 * Properties (Gradle: `-Pkrit.fir.bench.<name>=...`):
 *  - `files`: comma-separated sizes of synthetic modules (default 600);
 *  - `real`: comma-separated source directories checked as they are (copied first), instead of synthetic modules;
 *  - `edits`: measured edits per module (default 30), after 3 warm-up edits;
 *  - `policy`: `exclude` (default; tiered rules excluded) or `default` (all rules). No built-in rule is tiered any
 *    more, so both engage the planner; `default` is the production policy;
 *  - `hashes`: `true` (default) supplies content hashes as Go does; `false` makes the planner hash the module;
 *  - `fullEvery`: a full check is timed on every Nth edit (default so that about 10 are timed);
 *  - `classpath`: `test` (default for `real`) uses the test JVM's own libraries, else the stdlib only;
 *  - `out`: also append the table to this file.
 */
@Tag(IncrementalPlannerBench.TAG)
class IncrementalPlannerBench {
    companion object {
        const val TAG = "incremental-bench"
        private fun prop(name: String) = System.getProperty("krit.fir.bench.$name")
        private val SYNTHETIC_KINDS = listOf("body", "body", "comment", "sig", "overload", "add", "delete", "rename", "sealed", "shadow", "body")
    }

    @TempDir lateinit var tmp: Path

    private val out = StringBuilder()
    private fun say(line: String) { println(line); out.append(line).append('\n'); prop("out")?.let { File(it).appendText(line + "\n") } }

    private fun load() = try { File("/proc/loadavg").readText().split(" ").take(3).joinToString("/") } catch (_: Exception) { "?" }

    private fun pct(values: List<Long>, p: Double): Long {
        if (values.isEmpty()) return -1
        val s = values.sorted()
        return s[minOf(s.size - 1, Math.ceil(p * s.size).toInt() - 1).coerceAtLeast(0)]
    }

    /** One edit: what it touched, so the harness can name it. */
    private class Step(val kind: String)

    @Test fun bench() {
        val policy = when (prop("policy") ?: "exclude") {
            "default" -> BuiltInTieredPolicy.FULL_IF_TIERED_ENABLED
            else -> BuiltInTieredPolicy.EXCLUDE_TIERED
        }
        val edits = prop("edits")?.toInt() ?: 30
        val supplyHashes = prop("hashes") != "false"
        say("# bench policy=$policy edits=$edits hashes=$supplyHashes cpus=${Runtime.getRuntime().availableProcessors()} load(start)=${load()} (1/5/15 min)")
        val real = prop("real")
        if (real != null) {
            runModule("real:" + real.split(',').joinToString("+") { File(it).name }, real.split(',').map(::File), null, policy, edits, supplyHashes)
        } else {
            for (n in (prop("files") ?: "600").split(',').map { it.trim().toInt() }) {
                val root = tmp.resolve("synthetic-$n").toFile()
                val project = SyntheticProject(root, n).also { it.generate() }
                runModule("synthetic:$n", project.sourceDirs.map(::File), project, policy, edits, supplyHashes)
            }
        }
    }

    private fun classpathFor(real: Boolean): List<String> {
        if (!real || (prop("classpath") ?: "test") != "test") return listOf(testStdlib)
        // The libraries krit-fir's own sources compile against: the test JVM's jars, without the sources under test.
        return System.getProperty("java.class.path").split(File.pathSeparator).filter {
            it.endsWith(".jar") && !File(it).name.startsWith("krit-fir") && !File(it).name.startsWith("krit-rule-api")
        }
    }

    private fun runModule(name: String, dirs: List<File>, project: SyntheticProject?, policy: TieredRulePolicy, edits: Int, supplyHashes: Boolean) {
        val sourceDirs: List<File> = if (project != null) dirs else dirs.mapIndexed { i, d ->
            File(tmp.toFile(), "real-$i-${d.name}").also { target -> d.copyRecursively(target) }
        }
        val classpath = classpathFor(project == null)
        val planned = AnalysisSession(sourceDirs.map { it.path }, classpath, incrementalRecording = true, incrementalPlanning = true)
            .also { it.snapshotRootOverride = tmp.resolve("snapshots-$name".replace(':', '_')).toFile(); it.tieredRulePolicy = policy }
        val plain = AnalysisSession(sourceDirs.map { it.path }, classpath, incrementalRecording = false, incrementalPlanning = false)
        val rules = emptySet<String>() // "all rules": the policy decides which run
        val plainRules = if (policy == BuiltInTieredPolicy.EXCLUDE_TIERED)
            FirRuleDiscovery.rules.map { it.ruleId }.toSet() - FirRuleDiscovery.wholeCompilationRuleIds().toSet() else emptySet()

        fun files(): List<FileRef> = sourceDirs.flatMap { d -> d.walkTopDown().filter { it.isFile && it.extension == "kt" }.map { it.path } }.sorted()
            .map { FileRef(it, if (supplyHashes) sha1Hex(File(it).readBytes()) else "") }

        val rnd = Random(11)
        val realEdit = project == null
        val fileCount = files().size
        say("## $name: $fileCount files")
        val t0 = System.nanoTime()
        planned.check(0, files(), rules)
        say("cold (full check + snapshot): ${(System.nanoTime() - t0) / 1_000_000} ms; plan=${planned.lastPlan?.path} ${planned.lastPlan?.reason ?: ""}; " +
            "snapshot=${planned.incrementalState.snapshot != null} fallback=${planned.sliceFallback()} errors=${planned.verdicts.firstError(emptyList())?.take(300)}")

        val fullEvery = prop("fullEvery")?.toInt() ?: maxOf(1, edits / 10)
        val warm = 3
        class Sample(val kind: String, val incMs: Long, val fullMs: Long?, val path: PlanPath, val reason: FullReason?, val dirty: Int, val waves: Int, val fullFirst: Boolean)
        val samples = ArrayList<Sample>()
        val realState = RealEdits(sourceDirs, rnd)
        for (i in 1..(warm + edits)) {
            val kind = if (project != null) SYNTHETIC_KINDS[(i - 1) % SYNTHETIC_KINDS.size].also { project.apply(it) } else realState.next()
            val request = files()
            // The timed full check alternates between running before and after the planned one, so neither always
            // gets the other's warm caches.
            val timedFull = i > warm && (i - warm) % fullEvery == 0
            val fullFirst = timedFull && ((i - warm) / fullEvery) % 2 == 1
            var fullMs: Long? = null
            fun runFull() {
                val f0 = System.nanoTime()
                plain.check(i.toLong(), request, plainRules)
                fullMs = (System.nanoTime() - f0) / 1_000_000
            }
            if (fullFirst) runFull()
            val s0 = System.nanoTime()
            planned.check(i.toLong(), request, rules)
            val incMs = (System.nanoTime() - s0) / 1_000_000
            if (timedFull && !fullFirst) runFull()
            val plan = planned.lastPlan!!
            if (i > warm) samples += Sample(kind, incMs, fullMs, plan.path, plan.reason, plan.dirtyFiles, plan.waves, fullFirst)
            say("e$i %-12s planned=%6d ms %s%s dirty=%d waves=%d%s".format(kind, incMs, plan.path, plan.reason?.let { " ($it ${plan.detail})" } ?: "",
                plan.dirtyFiles, plan.waves, fullMs?.let { " full=$it ms (${if (fullFirst) "full first" else "planned first"})" } ?: ""))
        }
        val inc = samples.map { it.incMs }
        val full = samples.mapNotNull { it.fullMs }
        val engaged = samples.count { it.path == PlanPath.INCREMENTAL }
        say("RESULT $name files=$fileCount policy=$policy edits=${samples.size} engaged=$engaged fallbacks=${samples.filter { it.path == PlanPath.FULL }.groupingBy { it.reason }.eachCount()}")
        say("RESULT $name planned p50=${pct(inc, .5)} ms p95=${pct(inc, .95)} ms max=${inc.maxOrNull()} ms (n=${inc.size}); " +
            "full p50=${pct(full, .5)} ms p95=${pct(full, .95)} ms (n=${full.size}); load(end)=${load()}")
        for (kind in samples.map { it.kind }.distinct()) {
            val k = samples.filter { it.kind == kind }.map { it.incMs }
            say("RESULT $name   %-12s planned p50=%d p95=%d (n=%d)".format(kind, pct(k, .5), pct(k, .95), k.size))
        }
        planned.dispose(); plain.dispose()
    }

    /** Edits of real sources: a literal in a body, a comment, and a rename of a function used in other files (and back). */
    private class RealEdits(val dirs: List<File>, val rnd: Random) {
        private var renamed: Pair<String, String>? = null
        private val literalRe = Regex("""^(\s{8,}(?:val|var) \w+ = ")([^"\\\n$]{3,})(")\s*$""")

        private fun ktFiles() = dirs.flatMap { d -> d.walkTopDown().filter { it.isFile && it.extension == "kt" }.toList() }

        fun next(): String {
            renamed?.let { (from, to) -> replaceWord(to, from); renamed = null; return "renameBack" }
            return when (rnd.nextInt(4)) {
                0 -> comment()
                1 -> rename() ?: literal()
                else -> literal()
            }
        }

        private fun comment(): String {
            val all = ktFiles()
            val f = all[rnd.nextInt(all.size)]
            f.appendText("// edit ${rnd.nextInt()}\n")
            return "comment"
        }

        private fun literal(): String {
            val candidates = ktFiles().shuffled(rnd)
            for (f in candidates) {
                val lines = f.readLines().toMutableList()
                val idx = lines.indices.filter { literalRe.matches(lines[it]) }
                if (idx.isEmpty()) continue
                val i = idx[rnd.nextInt(idx.size)]
                lines[i] = literalRe.replace(lines[i]) { it.groupValues[1] + it.groupValues[2] + "x" + it.groupValues[3] }
                f.writeText(lines.joinToString("\n") + "\n")
                return "literal"
            }
            return comment()
        }

        /** Renames a top-level or object function that other files use. */
        private fun rename(): String? {
            val files = ktFiles()
            val texts = files.associateWith { it.readText() }
            val declRe = Regex("""^\s*(?:internal |public )?fun (?:<[^>]*> )?([a-z][A-Za-z0-9]{6,})\(""", RegexOption.MULTILINE)
            val names = texts.values.flatMap { declRe.findAll(it).map { m -> m.groupValues[1] }.toList() }
                .groupingBy { it }.eachCount().filterValues { it == 1 }.keys.toList().shuffled(rnd)
            for (name in names) {
                val wordRe = Regex("\\b$name\\b")
                val users = texts.filter { wordRe.containsMatchIn(it.value) }
                val overridden = texts.values.any { Regex("override fun $name\\b").containsMatchIn(it) }
                if (users.size < 2 || users.size > 8 || overridden) continue
                replaceWord(name, name + "R")
                renamed = name to name + "R"
                return "rename"
            }
            return null
        }

        private fun replaceWord(from: String, to: String) {
            val re = Regex("\\b$from\\b")
            for (f in ktFiles()) {
                val t = f.readText()
                if (re.containsMatchIn(t)) f.writeText(re.replace(t, to))
            }
        }
    }
}
