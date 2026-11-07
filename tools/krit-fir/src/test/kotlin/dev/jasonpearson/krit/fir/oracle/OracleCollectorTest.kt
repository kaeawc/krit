package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.cli.common.messages.CompilerMessageLocation
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class OracleCollectorTest {

    @TempDir
    lateinit var tmp: Path

    @Test
    fun smartCastDeclarationsAttachOnlyToTheVerdictsTheSmartCastDecides() {
        val content = "val x = (a.p)?.q\nval y = c.d + e.f?.g\nval z = a.p == null\nval w = a.p ?: b\n"
        val path = tmp.resolve("Use.kt").toFile().apply { writeText(content) }.absolutePath
        fun at(text: String, from: Int = 0) = content.indexOf(text, from).also { check(it >= 0) { text } }
        val c = OracleCollector()
        fun smartCast(start: Int, file: String) = c.addSmartCastDeclarations(path, start, start + 3, setOf(file))
        smartCast(at("a.p"), "/b/Model.kt")
        smartCast(at("c.d"), "/b/Other.kt")
        smartCast(at("a.p", at("val z")), "/b/Model.kt")
        smartCast(at("a.p", at("val w")), "/c/Third.kt")
        fun diagnostic(factory: String, start: Int, end: Int) =
            c.addDiagnostic(path, DiagnosticPayload(factory, "WARNING", factory, 1), start, end)
        diagnostic("UNNECESSARY_SAFE_CALL", at("?."), at("?.") + 2)
        diagnostic("UNNECESSARY_SAFE_CALL", at("?.g"), at("?.g") + 2)
        diagnostic("SENSELESS_COMPARISON", at("a.p == null"), at("a.p == null") + 11)
        diagnostic("USELESS_ELVIS", at("?: b"), at("?: b") + 4)
        diagnostic("UNREACHABLE_CODE", at("a.p"), at("a.p") + 3)
        c.addDiagnostic(path, DiagnosticPayload("USELESS_ELVIS", "WARNING", "no location", 1))

        val declarations = c.toResult().files.getValue(path).diagnostics.map { it.smartCastDeclarations }
        assertEquals(
            listOf(listOf("/b/Model.kt"), emptyList(), listOf("/b/Model.kt"), listOf("/c/Third.kt"), emptyList(), emptyList()),
            declarations,
        )
    }

    @Test
    fun smartCastDeclarationsRoundTripOnTheWire() {
        val result = AnalyzeResult(files = mapOf("/a/Use.kt" to FilePayload(diagnostics = listOf(
            DiagnosticPayload("UNNECESSARY_SAFE_CALL", "WARNING", "m", 2, 3, smartCastDeclarations = listOf("/b/Model.kt")),
            DiagnosticPayload("USELESS_ELVIS", "WARNING", "m", 4),
        ))))
        val json = OracleResponse.buildOneShot(result)
        assertTrue(""""smartCastDecls":["/b/Model.kt"]""" in json, json)
        assertEquals(1, Regex("smartCastDecls").findAll(json).count(), json)
    }

    @Test
    fun emptyCollectorProducesEmptyResult() {
        val result = OracleCollector().toResult()
        assertEquals(emptyMap(), result.files)
        assertEquals(emptyMap(), result.dependencies)
        assertEquals(emptyMap(), result.errors)
    }

    @Test
    fun addClassEmitsBothFileEntryAndDependencyEntry() {
        // The collector has a two-output shape: the
        // per-file `declarations` list AND the by-FQN `dependencies`
        // map are populated from the same class declaration. The Go
        // side uses one for per-file analysis and the other for
        // cross-file symbol resolution; both must agree on the payload.
        val c = OracleCollector()
        val payload = ClassPayload(fqn = "com.acme.Foo", kind = "class")
        c.addClass("/src/Foo.kt", payload)

        val result = c.toResult()
        assertEquals(listOf(payload), result.files["/src/Foo.kt"]?.declarations)
        assertEquals(payload, result.dependencies["com.acme.Foo"])
    }

    @Test
    fun multipleClassesInOneFileAccumulateInOrder() {
        val c = OracleCollector()
        val first = ClassPayload(fqn = "com.acme.First", kind = "class")
        val second = ClassPayload(fqn = "com.acme.Second", kind = "interface")
        c.addClass("/src/Foo.kt", first)
        c.addClass("/src/Foo.kt", second)

        val declarations = c.toResult().files["/src/Foo.kt"]?.declarations
        assertEquals(listOf(first, second), declarations)
    }

    @Test
    fun duplicateFqnAcrossFilesKeepsFirstSeenInDependencies() {
        // Two files declaring the same FQN is unusual but possible
        // during incremental analysis with stale sources. First-wins,
        // so the Go-side resolver sees a stable choice and doesn't see an unpredictable "last writer wins" race.
        val c = OracleCollector()
        val firstSeen = ClassPayload(fqn = "com.acme.Foo", kind = "class", visibility = "public")
        val secondSeen = ClassPayload(fqn = "com.acme.Foo", kind = "object", visibility = "internal")
        c.addClass("/src/A.kt", firstSeen)
        c.addClass("/src/B.kt", secondSeen)

        assertEquals(firstSeen, c.toResult().dependencies["com.acme.Foo"])
    }

    @Test
    fun packageDirectiveWithoutClassDeclarationsStillEmitsFileEntry() {
        // A Kotlin file with just `package` and top-level functions
        // (no classes) should still appear in the `files` map with
        // its package name — one file entry per visited file.
        val c = OracleCollector()
        c.setPackage("/src/Helpers.kt", "com.acme.util")

        val payload = c.toResult().files["/src/Helpers.kt"]
        assertEquals("com.acme.util", payload?.packageName)
        assertEquals(emptyList(), payload?.declarations)
    }

    @Test
    fun packageNameTrackedSeparatelyFromClassEntries() {
        val c = OracleCollector()
        c.setPackage("/src/Foo.kt", "com.acme")
        c.addClass("/src/Foo.kt", ClassPayload(fqn = "com.acme.Foo", kind = "class"))

        val payload = c.toResult().files["/src/Foo.kt"]
        assertEquals("com.acme", payload?.packageName)
        assertEquals(1, payload?.declarations?.size)
    }

    @Test
    fun addExpressionAccumulatesUnderFilePathAndKey() {
        val c = OracleCollector()
        val payload = ExpressionPayload(type = "", callTarget = "com.acme.foo")
        c.addExpression("/src/Foo.kt", "10:5", payload)

        val expressions = c.toResult().files["/src/Foo.kt"]?.expressions
        assertEquals(mapOf("10:5" to payload), expressions)
    }

    @Test
    fun duplicateExpressionKeysKeepFirstSeen() {
        // K2 can revisit the same call site during phased checking;
        // first-wins ("skip if key already present") keeps the wire
        // shape deterministic.
        val c = OracleCollector()
        val first = ExpressionPayload(type = "", callTarget = "first")
        val second = ExpressionPayload(type = "", callTarget = "second")
        c.addExpression("/src/Foo.kt", "1:1", first)
        c.addExpression("/src/Foo.kt", "1:1", second)

        assertEquals(first, c.toResult().files["/src/Foo.kt"]?.expressions?.get("1:1"))
        assertTrue(c.hasExpression("/src/Foo.kt", "1:1"))
    }

    @Test
    fun addDiagnosticAccumulatesPerFileInInsertionOrder() {
        val c = OracleCollector()
        val first = DiagnosticPayload(factoryName = "USELESS_ELVIS", severity = "WARNING", message = "a", line = 1)
        val second = DiagnosticPayload(factoryName = "CAST_NEVER_SUCCEEDS", severity = "WARNING", message = "b", line = 3)
        c.addDiagnostic("/src/Foo.kt", first)
        c.addDiagnostic("/src/Foo.kt", second)

        val diagnostics = c.toResult().files["/src/Foo.kt"]?.diagnostics
        assertEquals(listOf(first, second), diagnostics)
    }

    @Test
    fun diagnosticsForFileWithoutClassesStillEmitsFileEntry() {
        val c = OracleCollector()
        c.addDiagnostic(
            "/src/Only.kt",
            DiagnosticPayload(factoryName = "USELESS_ELVIS", severity = "WARNING", message = "x", line = 1),
        )
        val payload = c.toResult().files["/src/Only.kt"]
        assertEquals(1, payload?.diagnostics?.size)
        assertEquals(emptyList(), payload?.declarations)
        assertEquals(emptyMap(), payload?.expressions)
    }

    @Test
    fun compilerDiagnosticMessagesMapToNullSafetyFactories() {
        val messages = linkedMapOf(
            "Unnecessary non-null assertion (!!) on a non-null receiver of type 'String'." to
                "UNNECESSARY_NOT_NULL_ASSERTION",
            "Unnecessary safe call on a non-null receiver of type 'String'." to
                "UNNECESSARY_SAFE_CALL",
            "Condition is always 'true'." to "SENSELESS_COMPARISON",
            "Cast is redundant." to "USELESS_CAST",
        )

        for ((message, expectedFactory) in messages) {
            val collector = OracleCollector()
            val path = "/src/Diagnostics.kt"
            OracleDiagnosticMessageCollector(collector).report(
                CompilerMessageSeverity.WARNING,
                message,
                CompilerMessageLocation.create(path, 1, 1, message),
            )

            val diagnostics = collector.toResult().files[path]?.diagnostics.orEmpty()
            assertEquals(
                listOf(expectedFactory),
                diagnostics.map { it.factoryName },
                "unexpected mapping for: $message",
            )
        }
    }

    @Test
    fun elvisNullOperandMessagesAreNotAttributedToRetainedFactories() {
        val collector = OracleCollector()
        val path = "/src/Elvis.kt"
        val messageCollector = OracleDiagnosticMessageCollector(collector)
        val messages = listOf(
            "Elvis operator (?:) is useless if the left operand is null.",
            "Right operand of elvis operator (?:) is useless if it is null.",
        )

        for (message in messages) {
            messageCollector.report(
                CompilerMessageSeverity.WARNING,
                message,
                CompilerMessageLocation.create(path, 1, 1, message),
            )
        }

        assertEquals(emptyMap(), collector.toResult().files)
    }

    @Test
    fun expressionsForFileWithoutClassesStillEmitsFileEntry() {
        // A file that has only top-level function calls (no class
        // declarations and no package directive in the test) should
        // still appear in `files` with its expressions populated.
        val c = OracleCollector()
        c.addExpression("/src/Only.kt", "1:1", ExpressionPayload(type = "", callTarget = "x"))

        val payload = c.toResult().files["/src/Only.kt"]
        assertEquals(1, payload?.expressions?.size)
        assertEquals(emptyList(), payload?.declarations)
    }

    @Test
    fun registryScopesActiveCollectorPerThread() {
        // Sanity check on the thread-local guard. The orchestrator
        // assumes K2 runs single-threaded per compilation; the test
        // confirms a second `begin` overwrites the first, and `end`
        // clears.
        val first = OracleCollector()
        val second = OracleCollector()
        OracleCollectorRegistry.begin(first)
        assertTrue(OracleCollectorRegistry.current() === first)
        OracleCollectorRegistry.begin(second)
        assertTrue(OracleCollectorRegistry.current() === second)
        OracleCollectorRegistry.end()
        assertTrue(OracleCollectorRegistry.current() == null)
    }

    @Test
    fun smartCastTypeWinsRegardlessOfInsertionOrder() {
        // #666: after `if (x == null) return`, the smart-cast (non-null) type
        // must win over the declared `Any?` type at the same position, both
        // when FIR visits the smart cast first and when it visits it last.
        val declared = ExpressionPayload(type = "kotlin.Any", nullable = true)
        val smartCast = ExpressionPayload(type = "kotlin.Any", nullable = false)

        val smartFirst = OracleCollector()
        smartFirst.addSmartCastExpression("/src/A.kt", "3:5", smartCast)
        smartFirst.addExpression("/src/A.kt", "3:5", declared)
        assertEquals(smartCast, smartFirst.toResult().files["/src/A.kt"]?.expressions?.get("3:5"))

        val smartLast = OracleCollector()
        smartLast.addExpression("/src/A.kt", "3:5", declared)
        smartLast.addSmartCastExpression("/src/A.kt", "3:5", smartCast)
        assertEquals(smartCast, smartLast.toResult().files["/src/A.kt"]?.expressions?.get("3:5"))
    }

    @Test
    fun smartCastDoesNotReplaceCallSiteEntry() {
        val call = ExpressionPayload(type = "kotlin.String", callTarget = "com.acme.name", callTargetResolved = true)
        val smartCast = ExpressionPayload(type = "kotlin.String", nullable = false)
        val c = OracleCollector()
        c.addExpression("/src/A.kt", "4:9", call)
        c.addSmartCastExpression("/src/A.kt", "4:9", smartCast)
        assertEquals(call, c.toResult().files["/src/A.kt"]?.expressions?.get("4:9"))
    }

    @Test
    fun smartCastReceiverOfACallKeepsItsRefinedTypeUnderItsRange() {
        val call = ExpressionPayload(type = "kotlin.String", startByte = 10, endByte = 22, callTarget = "kotlin.Any.toString", callTargetResolved = true)
        val declared = ExpressionPayload(type = "kotlin.Any", nullable = true, startByte = 10, endByte = 11)
        val refined = ExpressionPayload(type = "kotlin.Any", nullable = false, startByte = 10, endByte = 11)
        val c = OracleCollector()
        c.addCallExpression("/src/A.kt", "2:5", call)
        c.addCallExpression("/src/A.kt", "2:5", declared)
        c.addSmartCastExpression("/src/A.kt", "2:5", refined)
        val expressions = c.toResult().files["/src/A.kt"]?.expressions
        assertEquals(mapOf("2:5" to call, "2:5@11" to refined), expressions)
    }

    @Test
    fun callsSharingAStartKeepBothRanges() {
        val outer = ExpressionPayload(type = "", startByte = 10, endByte = 40, callTarget = "p.outer", callTargetResolved = true)
        val inner = ExpressionPayload(type = "", startByte = 10, endByte = 22, callTarget = "p.inner", callTargetResolved = true, callTargetSuspend = true)
        val c = OracleCollector()
        c.addCallExpression("/src/A.kt", "2:5", outer)
        assertFalse(c.hasExpressionRange("/src/A.kt", "2:5", 10, 22))
        c.addCallExpression("/src/A.kt", "2:5", inner)
        // A revisit of the same call is not recorded twice.
        c.addCallExpression("/src/A.kt", "2:5", inner.copy(callTarget = "p.revisit"))
        assertTrue(c.hasExpressionRange("/src/A.kt", "2:5", 10, 22))
        val expressions = c.toResult().files["/src/A.kt"]?.expressions
        assertEquals(mapOf("2:5" to outer, "2:5@22" to inner), expressions)
    }
}
