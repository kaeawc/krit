package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.Assumptions.assumeTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals

/**
 * A class from a module whose build output is missing resolves as an error type. The compiler treats it as non-null,
 * as a subtype of everything and as `Nothing`, so the warnings that rest on it describe its guess, not the program.
 * Each case is one function on one line: a `drop` case must report nothing, a `keep` case must keep its verdict.
 */
class ErrorTypeDiagnosticsTest {

    @TempDir
    lateinit var tmp: Path

    private lateinit var stdlibClasspath: List<String>

    @BeforeEach
    fun resolveStdlib() {
        val stdlib = findKotlinStdlib()
        assumeTrue(stdlib != null, "kotlin-stdlib jar not found; set KOTLIN_STDLIB_JAR")
        stdlibClasspath = listOfNotNull(stdlib)
    }

    private class Case(val name: String, val source: String, val keeps: String?)

    private val header = """
        package com.acme.app

        import com.acme.missing.Missing
        import com.acme.missing.Other
        import com.acme.missing.loadIt

        class Holder(val m: Missing, val mn: Missing?) {
            val derived = m.next
        }
        fun inSource(): Missing = TODO()
        fun generic(): List<Missing> = emptyList()
        fun <T> pick(a: T, b: T): T = a
        fun takes(vararg values: Any?) {}
    """.trimIndent()

    private val cases = listOf(
        // USELESS_ELVIS, SENSELESS_COMPARISON, UNNECESSARY_NOT_NULL_ASSERTION, UNNECESSARY_SAFE_CALL, USELESS_CAST.
        Case("keepElvis", "fun keepElvis(s: String): String = s ?: \"d\"", "USELESS_ELVIS"),
        Case("keepComparison", "fun keepComparison(s: String): Boolean = s == null", "SENSELESS_COMPARISON"),
        Case("keepAssertion", "fun keepAssertion(s: String) = s!!", "UNNECESSARY_NOT_NULL_ASSERTION"),
        Case("keepSafeCall", "fun keepSafeCall(s: String) = s?.length", "UNNECESSARY_SAFE_CALL"),
        Case("keepCast", "fun keepCast(s: String) = s as String", "USELESS_CAST"),
        Case("keepNeverCast", "fun keepNeverCast(s: String) = s as Int", "CAST_NEVER_SUCCEEDS"),
        Case("keepUnitAssertion", "fun keepUnitAssertion(h: Holder, fn: (Missing) -> Unit) = fn(h.m)!!", "UNNECESSARY_NOT_NULL_ASSERTION"),
        Case("keepNestedSafeCall", "fun keepNestedSafeCall(h: Holder, s: String) = takes(h.m, s?.length)", "UNNECESSARY_SAFE_CALL"),
        // An `is` check proves the value non-null whatever the checked type is.
        Case("keepIsCheckAssertion", "fun keepIsCheckAssertion(o: Any) = if (o is Missing) o!! else 0", "UNNECESSARY_NOT_NULL_ASSERTION"),
        Case("dropParamAssertion", "fun dropParamAssertion(m: Missing) = m!!", null),
        Case("dropParamSafeCall", "fun dropParamSafeCall(m: Missing) = m?.name", null),
        Case("dropPropertyAssertion", "fun dropPropertyAssertion(h: Holder) = h.m!!", null),
        Case("dropPropertySafeCall", "fun dropPropertySafeCall(h: Holder) = h.m?.x", null),
        Case("dropInferredAssertion", "fun dropInferredAssertion(h: Holder) = h.derived!!", null),
        Case("dropCallAssertion", "fun dropCallAssertion() = inSource()!!", null),
        Case("dropCallSafeCall", "fun dropCallSafeCall() = inSource()?.x", null),
        Case("dropGenericAssertion", "fun dropGenericAssertion() = generic().first()!!", null),
        Case("dropLocal", "fun dropLocal(h: Holder) { val v = h.m; v!!; v?.x; v ?: 1; v == null }", null),
        Case("dropCastAssertion", "fun dropCastAssertion(o: Any) = (o as Missing)!!", null),
        Case("dropElvisAssertion", "fun dropElvisAssertion(h: Holder) = (h.m ?: h.m)!!", null),
        Case("dropGenericCallAssertion", "fun dropGenericCallAssertion(h: Holder) = pick(h.m, h.m)!!", null),
        Case("dropCastToMissing", "fun dropCastToMissing(s: String) = (s as Missing)!!", null),
        // DEPRECATION resolves a member on an error receiver against any declaration of that name.
        Case("keepDeprecated", "fun keepDeprecated() = 'a'.toInt()", "DEPRECATION"),
        Case("keepDeprecatedInArgs", "fun keepDeprecatedInArgs(m: Missing) = takes(m.x, 'a'.toInt())", "DEPRECATION"),
        Case("dropDeprecatedReceiver", "fun dropDeprecatedReceiver(m: Missing) = m.str.capitalize()", null),
        Case("dropDeprecatedChain", "fun dropDeprecatedChain(h: Holder) = h.m.str.decapitalize()", null),
        // UNREACHABLE_CODE after a statement whose type comes from an error type.
        Case("keepDead", "fun keepDead(m: Missing): Int { return m.value; println(\"dead\") }", "UNREACHABLE_CODE"),
        Case("dropTryCatch", "fun dropTryCatch(): Int { val s = try { loadIt() } catch (e: IllegalStateException) { return 1 }; return s }", null),
        Case("dropTryCatchLocal", "fun dropTryCatchLocal(): Int { val s = try { load() } catch (e: IllegalStateException) { return 1 }; return s }", null),
        Case("dropIfElseReturn", "fun dropIfElseReturn(m: Missing): Int { val y = if (m.ok) m.a else return 0; println(y); return 1 }", null),
        Case("dropWhenReturn", "fun dropWhenReturn(m: Missing): Int { val y = when (m.kind) { 1 -> m.a else -> return 0 }; println(y); return 1 }", null),
    )

    @Test
    fun errorTypedOperandsDoNotKeepCompilerVerdicts() {
        val lines = header.lines().toMutableList()
        val caseLines = LinkedHashMap<Int, Case>()
        for (case in cases) {
            lines += case.source
            caseLines[lines.size] = case
        }
        val path = tmp.resolve("app/Cases.kt").toFile().also { it.parentFile.mkdirs() }.apply { writeText(lines.joinToString("\n")) }
        val result = AnalysisSession(sourceDirs = listOf(tmp.toFile().absolutePath), classpath = stdlibClasspath).analyze(emptyList())
        val diagnostics = result.files[path.absolutePath]?.diagnostics.orEmpty().groupBy { it.line }
        val problems = caseLines.mapNotNull { (line, case) ->
            val got = diagnostics[line].orEmpty().map { it.factoryName }.toSet()
            val want = setOfNotNull(case.keeps)
            if (got == want) null else "${case.name}: expected $want, got $got"
        }
        assertEquals(emptyList(), problems)
    }

    @Test
    fun callsLookedUpOnAnErrorReceiverAreReportedUnresolved() {
        val source = """
            package com.acme.app

            import com.acme.missing.Missing

            fun viaError(m: Missing) = m.str.capitalize()
            fun viaStdlib(s: String) = s.capitalize()
        """.trimIndent()
        val path = tmp.resolve("app/Calls.kt").toFile().also { it.parentFile.mkdirs() }.apply { writeText(source) }
        val result = AnalysisSession(sourceDirs = listOf(tmp.toFile().absolutePath), classpath = stdlibClasspath).analyze(emptyList())
        val calls = result.files[path.absolutePath]?.expressions.orEmpty().values.filter { it.callTarget?.endsWith("capitalize") == true }
        // The extension the compiler picked for the error receiver is a guess: it is reported unresolved and untyped.
        val guessed = calls.filter { !it.callTargetResolved }
        val resolved = calls.filter { it.callTargetResolved }
        assertEquals(listOf("capitalize"), guessed.map { it.callTarget })
        assertEquals(listOf("<error>"), guessed.map { it.type })
        assertEquals(listOf("kotlin.text.capitalize"), resolved.map { it.callTarget })
    }

    private fun findKotlinStdlib(): String? {
        System.getenv("KOTLIN_STDLIB_JAR")?.let { override -> if (File(override).isFile) return override }
        val home = System.getProperty("user.home") ?: return null
        val cacheRoot = File(home, ".gradle/caches/modules-2/files-2.1/org.jetbrains.kotlin/kotlin-stdlib")
        if (!cacheRoot.isDirectory) return null
        return cacheRoot.walkTopDown()
            .filter { it.isFile && it.name.startsWith("kotlin-stdlib-") && it.name.endsWith(".jar") }
            .filter { !it.name.contains("sources") && !it.name.contains("javadoc") }
            .sortedByDescending { it.name }
            .firstOrNull()?.absolutePath
    }
}
