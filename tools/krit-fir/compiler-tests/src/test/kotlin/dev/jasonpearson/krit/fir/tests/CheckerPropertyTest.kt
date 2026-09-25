package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.fail

// Property tests over the FIR checkers. Each generator sweeps a bounded grid
// across the axes that decide a rule's verdict, and every case carries its
// expected verdict. Because the compile runs in-process (no subprocess), the
// whole grid for a rule compiles in one KritFirProbe call and is asserted per
// file. Metamorphic cases assert that a verdict-flipping edit actually flips it.
class CheckerPropertyTest {

    private data class Case(val name: String, val source: String, val shouldFlag: Boolean, val axes: String)

    // Runs every case in one compile and asserts each file flags iff shouldFlag.
    private fun checkGrid(diag: String, cases: List<Case>) {
        val diags = KritFirProbe.diagnose(cases.associate { it.name to it.source })
        val counts = cases.associate { c ->
            c.name to diags.count { it.file == c.name && it.name == diag }
        }
        val failures = buildList {
            for (c in cases) {
                val n = counts.getValue(c.name)
                if (c.shouldFlag && n == 0) add("[${c.name}] expected $diag (${c.axes}) but got none — false negative")
                if (!c.shouldFlag && n != 0) add("[${c.name}] expected no $diag (${c.axes}) but got $n — false positive")
            }
        }
        if (failures.isNotEmpty()) fail(failures.joinToString("\n"))
    }

    // ---- Compose: capture × keyed ----

    private fun composeGrid(): List<Case> {
        val calc = mapOf(
            "noCapture" to Pair("remember { 42 }", "remember(seed) { 42 }"),
            "lambdaParam" to Pair("remember { seed + 1 }", "remember(seed) { seed + 1 }"),
            "callableRefParam" to Pair("remember(seed::toString)", "remember(seed, seed::toString)"),
        )
        return buildList {
            for ((capture, forms) in calc) {
                for (keyed in listOf(false, true)) {
                    val name = "cp_${capture}_keyed$keyed.kt"
                    val shouldFlag = !keyed && capture != "noCapture"
                    val call = if (keyed) forms.second else forms.first
                    add(
                        Case(
                            name,
                            """
                            package cp_${capture}_$keyed
                            import androidx.compose.runtime.remember
                            fun Screen(seed: Int) {
                                val v = $call
                                use(v)
                            }
                            fun use(x: Any?) {}
                            """.trimIndent(),
                            shouldFlag,
                            "capture=$capture keyed=$keyed",
                        ),
                    )
                }
            }
        }
    }

    // ---- Flow: lifecycle method × wrapper (collect needs a coroutine; repeatOnLifecycle is the safe wrapper) ----

    private fun flowGrid(): List<Case> {
        val methods = listOf("onCreate", "onStart", "onViewCreated", "onResume", "observe")
        val lifecycle = setOf("onCreate", "onStart", "onViewCreated")
        val wrappers = listOf(
            // collect is suspend, so every case launches a coroutine first.
            // repeatOnLifecycle is the one safe wrapper; launchWhenStarted only
            // suspends the collector, so it does not clear the finding.
            Triple("none", "owner.lifecycleScope.launch {", "}"),
            Triple(
                "repeatOnLifecycle",
                "owner.lifecycleScope.launch { owner.repeatOnLifecycle(Lifecycle.State.STARTED) {",
                "} }",
            ),
            Triple("launchWhenStarted", "owner.lifecycleScope.launchWhenStarted {", "}"),
        )
        return buildList {
            for (m in methods) {
                for ((w, open, close) in wrappers) {
                    val name = "fl_${m}_$w.kt"
                    val shouldFlag = m in lifecycle && w != "repeatOnLifecycle"
                    add(
                        Case(
                            name,
                            """
                            package fl_${m}_$w
                            import androidx.lifecycle.Lifecycle
                            import androidx.lifecycle.LifecycleOwner
                            import androidx.lifecycle.lifecycleScope
                            import androidx.lifecycle.repeatOnLifecycle
                            import kotlinx.coroutines.flow.Flow
                            import kotlinx.coroutines.launch
                            class F(private val owner: LifecycleOwner) {
                                private val flow: Flow<Int> = TODO()
                                fun $m() {
                                    $open
                                    flow.collect { println(it) }
                                    $close
                                }
                            }
                            """.trimIndent(),
                            shouldFlag,
                            "method=$m wrapper=$w",
                        ),
                    )
                }
            }
        }
    }

    // ---- InjectDispatcher: owner × dispatcher ----

    private fun injectGrid(): List<Case> {
        data class Owner(val key: String, val flagged: Boolean, val wrap: (String) -> String)
        val owners = listOf(
            Owner("member", true) { b -> "class C {\n    suspend fun m() { $b }\n}" },
            Owner("object", true) { b -> "object O {\n    suspend fun m() { $b }\n}" },
            Owner("companion", true) { b -> "class C {\n    companion object {\n        suspend fun m() { $b }\n    }\n}" },
            Owner("topLevel", false) { b -> "suspend fun m() { $b }" },
            Owner("extension", false) { b -> "suspend fun String.m() { $b }" },
        )
        val dispatchers = listOf("IO", "Default", "Unconfined", "Main")
        return buildList {
            for (o in owners) {
                for (d in dispatchers) {
                    val name = "inj_${o.key}_$d.kt"
                    val shouldFlag = o.flagged && d != "Main"
                    add(
                        Case(
                            name,
                            """
                            package inj_${o.key}_$d
                            import kotlinx.coroutines.Dispatchers
                            import kotlinx.coroutines.withContext
                            ${o.wrap("withContext(Dispatchers.$d) { }")}
                            """.trimIndent(),
                            shouldFlag,
                            "owner=${o.key} dispatcher=$d",
                        ),
                    )
                }
            }
        }
    }

    // ---- UnsafeCastWhenNullable: operand × op × target ----

    private fun castGrid(): List<Case> {
        data class Operand(val key: String, val decl: String, val expr: String, val subtype: Boolean)
        val operands = listOf(
            Operand("anyNullable", "val a: Any? = any()", "a", false),
            Operand("nonNullString", "val a: String = \"\"", "a", true),
            Operand("nullLiteral", "", "null", true),
        )
        val ops = listOf(Triple("as", "as", true), Triple("asSafe", "as?", false))
        val targets = listOf(Triple("nullable", "String?", true), Triple("nonNull", "String", false))
        return buildList {
            for (o in operands) {
                for ((opKey, opTok, isAs) in ops) {
                    for ((tKey, tTyp, tNullable) in targets) {
                        if (!isAs && tNullable) continue // `as? String?` is not valid Kotlin
                        val name = "ct_${o.key}_${opKey}_$tKey.kt"
                        val shouldFlag = isAs && tNullable && !o.subtype
                        val declLine = if (o.decl.isEmpty()) "" else "${o.decl}\n    "
                        add(
                            Case(
                                name,
                                """
                                package ct_${o.key}_${opKey}_$tKey
                                fun any(): Any? = null
                                fun f() {
                                    ${declLine}val y = ${o.expr} $opTok $tTyp
                                    use(y)
                                }
                                fun use(x: Any?) {}
                                """.trimIndent(),
                                shouldFlag,
                                "operand=${o.key} op=$opKey target=$tKey",
                            ),
                        )
                    }
                }
            }
        }
    }

    @Test fun composeExpectation() = checkGrid("ComposeRememberWithoutKey", composeGrid())

    @Test fun flowExpectation() = checkGrid("CollectInOnCreateWithoutLifecycle", flowGrid())

    @Test fun injectExpectation() = checkGrid("InjectDispatcher", injectGrid())

    @Test fun castExpectation() = checkGrid("UnsafeCastWhenNullable", castGrid())

    // Metamorphic: a verdict-flipping edit must flip the verdict. Each pair is a
    // flagged source and the same code with the one change that should clear it.
    @Test
    fun metamorphicClearingEdits() {
        data class Pair3(val name: String, val diag: String, val flagged: String, val cleared: String)
        val mm2Imports = listOf(
            "androidx.lifecycle.Lifecycle",
            "androidx.lifecycle.LifecycleOwner",
            "androidx.lifecycle.lifecycleScope",
            "androidx.lifecycle.repeatOnLifecycle",
            "kotlinx.coroutines.flow.Flow",
            "kotlinx.coroutines.launch",
        ).joinToString("\n") { "import $it" }
        val pairs = listOf(
            Pair3(
                "compose_addKey", "ComposeRememberWithoutKey",
                "package mm1\nimport androidx.compose.runtime.remember\nfun S(seed: Int) { val v = remember { seed + 1 }; use(v) }\nfun use(x: Any?) {}",
                "package mm1\nimport androidx.compose.runtime.remember\nfun S(seed: Int) { val v = remember(seed) { seed + 1 }; use(v) }\nfun use(x: Any?) {}",
            ),
            Pair3(
                "flow_wrapRepeatOnLifecycle", "CollectInOnCreateWithoutLifecycle",
                "package mm2\n$mm2Imports\nclass F(val owner: LifecycleOwner) { val flow: Flow<Int> = TODO(); fun onCreate() { owner.lifecycleScope.launch { flow.collect { } } } }",
                "package mm2\n$mm2Imports\nclass F(val owner: LifecycleOwner) { val flow: Flow<Int> = TODO(); fun onCreate() { owner.lifecycleScope.launch { owner.repeatOnLifecycle(Lifecycle.State.STARTED) { flow.collect { } } } } }",
            ),
            Pair3(
                "inject_toParameter", "InjectDispatcher",
                "package mm3\nimport kotlinx.coroutines.Dispatchers\nimport kotlinx.coroutines.withContext\nclass C { suspend fun m() { withContext(Dispatchers.IO) { } } }",
                "package mm3\nimport kotlinx.coroutines.CoroutineDispatcher\nimport kotlinx.coroutines.withContext\nclass C { suspend fun m(d: CoroutineDispatcher) { withContext(d) { } } }",
            ),
            Pair3(
                "cast_toSafeCast", "UnsafeCastWhenNullable",
                "package mm4\nfun any(): Any? = null\nfun f() { val a: Any? = any(); val y = a as String?; use(y) }\nfun use(x: Any?) {}",
                "package mm4\nfun any(): Any? = null\nfun f() { val a: Any? = any(); val y = a as? String; use(y) }\nfun use(x: Any?) {}",
            ),
        )
        val failures = buildList {
            for (p in pairs) {
                val flaggedDiags = KritFirProbe.diagnose(p.flagged).count { it.name == p.diag }
                val clearedDiags = KritFirProbe.diagnose(p.cleared).count { it.name == p.diag }
                if (flaggedDiags == 0) add("[${p.name}] flagged variant did not emit ${p.diag}")
                if (clearedDiags != 0) add("[${p.name}] cleared variant still emits ${p.diag} ($clearedDiags)")
            }
        }
        if (failures.isNotEmpty()) fail(failures.joinToString("\n"))
    }
}
