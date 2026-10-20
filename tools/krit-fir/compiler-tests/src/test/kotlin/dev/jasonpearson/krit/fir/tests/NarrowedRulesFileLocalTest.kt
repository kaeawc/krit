package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// The 15 rules that read other declarations' bodies and initializers follow them only in the file under check,
// and from other files use what class files keep: const values, signatures and types (#873). An incremental
// slice compiles the changed files from source and takes the rest of the module from class files, so a rule
// that read a body of another file would give a different verdict there. For each rule:
//  - `sameFile` keeps the fact in the file that uses it: the rule must fire (or stay quiet) as before;
//  - `otherFile` moves the fact to another file: the verdict is the one a class file gives, which is what a
//    slice compile sees, and which the split differential (FirIncrementalSplitDifferentialTest) checks against
//    a real slice.
class NarrowedRulesFileLocalTest {

    private fun diags(rule: String, sources: Map<String, String>, options: Map<String, Any?>? = null) =
        KritFirProbe.compile(
            sources,
            FirRuleCompileContext(
                enabledRuleIds = setOf(rule),
                ruleConfigs = options?.let { mapOf(rule to it) }.orEmpty(),
            ),
        ).also { assertTrue(it.clean, it.problems()) }.diags.filter { it.name == rule }

    private fun findings(rule: String, sources: Map<String, String>, options: Map<String, Any?>? = null) =
        diags(rule, sources, options).map { it.file to it.line }.sortedWith(compareBy({ it.first }, { it.second }))

    private fun one(text: String) = mapOf("All.kt" to text)

    // HardcodedSecretKey: a key held by a non-const val.
    @Test fun hardcodedSecretKey() {
        val rule = "HardcodedSecretKey"
        val use = "val key = SecretKeySpec(java.util.Base64.getDecoder().decode(Keys.K1), \"AES\")"
        val header = "package p\nimport javax.crypto.spec.SecretKeySpec\n"
        val keys = "object Keys { val K1: String = Chain.K2 }\nobject Chain { val K2: String = \"c2VjcmV0a2V5MTIzNDU2Nw==\" }\n"
        assertEquals(listOf("All.kt" to 7), findings(rule, one("$header\n$keys\n$use\n")))
        assertEquals(
            emptyList(),
            findings(rule, mapOf("Keys.kt" to "package p\n$keys", "Use.kt" to "$header\n$use\n")),
        )
        // A const survives in a class file, so it is followed across files.
        assertEquals(
            listOf("Use.kt" to 4),
            findings(
                rule,
                mapOf(
                    "Keys.kt" to "package p\nobject Keys { const val K1 = \"c2VjcmV0a2V5MTIzNDU2Nw==\" }\n",
                    "Use.kt" to "$header\n$use\n",
                ),
            ),
        )
    }

    // StaticIv: an IV built from a string val.
    @Test fun staticIv() {
        val rule = "StaticIv"
        val header = "package p\nimport javax.crypto.spec.IvParameterSpec\n"
        val holder = "object B { val P: String = \"\${C.Q}\" }\nobject C { val Q: String = \"0123456789abcdef\" }\n"
        val use = "fun iv() = IvParameterSpec(\"\${B.P}\".toByteArray())"
        assertEquals(listOf("All.kt" to 7), findings(rule, one("$header\n$holder\n$use\n")))
        assertEquals(
            emptyList(),
            findings(rule, mapOf("B.kt" to "package p\n$holder", "A.kt" to "$header\n$use\n")),
        )
    }

    // WeakKeySize: the generator held by a val.
    @Test fun weakKeySize() {
        val rule = "WeakKeySize"
        val gen = "import javax.crypto.KeyGenerator\nval gen: KeyGenerator = KeyGenerator.getInstance(\"AES\")\n"
        val use = "fun weak() { gen.init(64) }\n"
        assertEquals(listOf("All.kt" to 5), findings(rule, one("package p\n$gen\n$use")))
        assertEquals(emptyList(), findings(rule, mapOf("Gen.kt" to "package p\n$gen", "Use.kt" to "package p\n$use")))
        // The algorithm of a const is a value a class file keeps.
        assertEquals(
            listOf("Use.kt" to 6),
            findings(
                rule,
                mapOf(
                    "Algs.kt" to "package p\nconst val ALG = \"AES\"\n",
                    "Use.kt" to "package p\nimport javax.crypto.KeyGenerator\n\nfun weak() {\n    val g = KeyGenerator.getInstance(ALG)\n    g.init(64)\n}\n",
                ),
            ),
        )
    }

    // LongLogTag: a tag chain, and the spelling of a const tag with an escape sequence.
    @Test fun longLogTag() {
        val rule = "LongLogTag"
        val header = "package p\nimport android.util.Log\n"
        val chain = "object B { val TAG: String = C.TAG }\nobject C { val TAG: String = \"AVeryLongTagThatExceedsTheLimit\" }\n"
        val use = "fun log() { Log.d(B.TAG, \"m\") }"
        assertEquals(listOf("All.kt" to 7), findings(rule, one("$header\n$chain\n$use\n")))
        assertEquals(
            emptyList(),
            findings(rule, mapOf("B.kt" to "package p\n$chain", "A.kt" to "$header\n$use\n")),
        )
    }

    // The message of a const tag declared in another file spells the constant's value: a class file has no source
    // text. The same literal in the file under check is spelled as written, as Go prints it.
    @Test fun longLogTagEscapeSequenceIsSpelledByValueAcrossFiles() {
        val rule = "LongLogTag"
        val const = "object Tags { const val T = \"Tag\\u0041VeryLongTagThatExceedsTheLimit\" }\n"
        val use = "package p\nimport android.util.Log\nfun log() { Log.d(Tags.T, \"m\") }\n"
        val across = diags(rule, mapOf("Tags.kt" to "package p\n$const", "A.kt" to use)).single()
        assertEquals("A.kt", across.file)
        assertTrue(across.message.contains("\"TagAVeryLongTagThatExceedsTheLimit\""), across.message)
        val sameFile = diags(rule, one("$use$const")).single()
        assertTrue(sameFile.message.contains("\"Tag\\u0041VeryLongTagThatExceedsTheLimit\""), sameFile.message)
    }

    // LocaleGetDefaultForFormatting: a formatter forwarded by vals.
    @Test fun localeGetDefaultForFormatting() {
        val rule = "LocaleGetDefaultForFormatting"
        val header = "package p\nimport java.time.format.DateTimeFormatter\nimport java.util.Locale\n"
        val holder = "val F0: DateTimeFormatter = DateTimeFormatter.ISO_DATE\nval F1: DateTimeFormatter = F0\n"
        val use = "fun fmt() = F1.withLocale(Locale.getDefault())"
        assertEquals(listOf("All.kt" to 8), findings(rule, one("$header\n$holder\n$use\n")))
        assertEquals(
            emptyList(),
            findings(
                rule,
                mapOf(
                    "F.kt" to "package p\nimport java.time.format.DateTimeFormatter\n$holder",
                    "A.kt" to "$header\n$use\n",
                ),
            ),
        )
    }

    // JdbcStatementExecute: a val is static only in the file under check (a const is static everywhere).
    @Test fun jdbcStatementExecute() {
        val rule = "JdbcStatementExecute"
        val header = "package p\nimport java.sql.Statement\n"
        val use = "fun run(s: Statement) { s.execute(QUERY) }"
        val computed = "fun run(s: Statement, t: String) { s.execute(\"SELECT \" + t) }"
        assertEquals(listOf("All.kt" to 4), findings(rule, one("$header\n$computed\n")))
        assertEquals(emptyList(), findings(rule, one("$header\nval QUERY: String = \"SELECT 1\"\n$use\n")))
        // Go treats a name it cannot see as computed, and a class file keeps no initializer.
        assertEquals(
            listOf("A.kt" to 4),
            findings(rule, mapOf("B.kt" to "package p\nval QUERY: String = \"SELECT 1\"\n", "A.kt" to "$header\n$use\n")),
        )
        assertEquals(
            emptyList(),
            findings(rule, mapOf("B.kt" to "package p\nconst val QUERY = \"SELECT 1\"\n", "A.kt" to "$header\n$use\n")),
        )
    }

    // CollectionsSynchronizedListIteration: a val holding the wrapper.
    @Test fun collectionsSynchronizedListIteration() {
        val rule = "CollectionsSynchronizedListIteration"
        val holder = "import java.util.Collections\nval items: MutableList<String> = Collections.synchronizedList(ArrayList())\n"
        val use = "fun dump() { for (x in items) println(x) }\n"
        assertEquals(listOf("All.kt" to 5), findings(rule, one("package p\n$holder\n$use")))
        assertEquals(
            emptyList(),
            findings(rule, mapOf("B.kt" to "package p\n$holder", "A.kt" to "package p\n$use")),
        )
    }

    // SharedFlowWithoutReplay: a factory function returning a default flow.
    @Test fun sharedFlowWithoutReplay() {
        val rule = "SharedFlowWithoutReplay"
        val header = "package p\nimport kotlinx.coroutines.flow.MutableSharedFlow\n"
        val factory = "fun <T> factory(): MutableSharedFlow<T> = MutableSharedFlow()\n"
        val use = "class Vm { val events = factory<Int>() }\n"
        assertEquals(listOf("All.kt" to 6), findings(rule, one("$header\n$factory\n$use")))
        assertEquals(
            emptyList(),
            findings(rule, mapOf("B.kt" to "$header$factory", "A.kt" to "package p\n$use")),
        )
    }

    // ShortAlarm: a project wrapper is read in the file under check; one of another file is reported like a
    // library function (Go matches the name).
    @Test fun shortAlarm() {
        val rule = "ShortAlarm"
        val header = "package p\nimport android.app.AlarmManager\nimport android.app.PendingIntent\n"
        val direct = "fun f(am: AlarmManager, pi: PendingIntent) { am.setRepeating(0, 0L, 5000L, pi) }\n"
        assertEquals(listOf("All.kt" to 5), findings(rule, one("$header\n$direct")))
        val wrapper = "object Sched { fun setRepeating(t: Int, at: Long, every: Long, pi: PendingIntent) { println(t) } }\n"
        val use = "fun g(pi: PendingIntent) { Sched.setRepeating(0, 0L, 5000L, pi) }\n"
        assertEquals(emptyList(), findings(rule, one("$header\n$wrapper$use")))
        assertEquals(
            listOf("A.kt" to 5),
            findings(rule, mapOf("B.kt" to "$header$wrapper", "A.kt" to "$header\n$use")),
        )
        // A const interval is a value a class file keeps, evaluated.
        assertEquals(
            listOf("A.kt" to 5),
            findings(
                rule,
                mapOf(
                    "B.kt" to "package p\nconst val EVERY = 5 * 1000L\n",
                    "A.kt" to "$header\nfun h(am: AlarmManager, pi: PendingIntent) { am.setRepeating(0, 0L, EVERY, pi) }\n",
                ),
            ),
        )
    }

    // WorldWriteableFiles: a project variable named like the constant.
    @Test fun worldWriteableFiles() {
        val rule = "WorldWriteableFiles"
        val holder = "object B { val MODE_WORLD_WRITEABLE: Int = 0 }\n"
        val use = "fun mode() = B.MODE_WORLD_WRITEABLE\n"
        // In the file under check its value is known and safe.
        assertEquals(emptyList(), findings(rule, one("package p\n$holder$use")))
        // In another file the value is unknown: reported, as Go reports the name.
        assertEquals(
            listOf("A.kt" to 2),
            findings(rule, mapOf("B.kt" to "package p\n$holder", "A.kt" to "package p\n$use")),
        )
        // A const keeps its value in a class file.
        assertEquals(
            emptyList(),
            findings(
                rule,
                mapOf("B.kt" to "package p\nconst val MODE_WORLD_WRITEABLE = 1 shl 2\n", "A.kt" to "package p\nfun mode() = MODE_WORLD_WRITEABLE\n"),
            ),
        )
        assertEquals(
            listOf("A.kt" to 2),
            findings(
                rule,
                mapOf("B.kt" to "package p\nconst val MODE_WORLD_WRITEABLE = 1 shl 1\n", "A.kt" to "package p\nfun mode() = MODE_WORLD_WRITEABLE\n"),
            ),
        )
    }

    // SecureRandom: a Random-typed val holding a SecureRandom.
    @Test fun secureRandom() {
        val rule = "SecureRandom"
        val holder = "import java.security.SecureRandom\nimport java.util.Random\nval rng: Random = SecureRandom()\n"
        val use = "fun seed() { rng.setSeed(1L) }\n"
        assertEquals(listOf("All.kt" to 5), findings(rule, one("package p\n$holder$use")))
        assertEquals(
            emptyList(),
            findings(rule, mapOf("B.kt" to "package p\n$holder", "A.kt" to "package p\n$use")),
        )
    }

    // ShowToast: a property assigned in the file under check. One declared in another file is followed as a class
    // file has it, with no initializer, so a toast shown through it counts as shown.
    @Test fun showToast() {
        val rule = "ShowToast"
        val header = "package p\nimport android.widget.Toast\nimport android.content.Context\n"
        val property = "lateinit var ctx: Context\nvar saved: Toast? = Toast.makeText(ctx, \"x\", 0)\n"
        val use = "fun g(c: Context) {\n    saved = Toast.makeText(c, \"y\", 0)\n    saved?.show()\n}\n"
        // A property with a non-null initializer in this file is not followed: the assigned toast is reported
        // (line 8: the header is 3 lines, the property 2, then a blank line and `fun g`).
        assertTrue(("All.kt" to 8) in findings(rule, one("$header$property\n$use")))
        // The toast bare statement still fires with no property involved.
        assertEquals(listOf("All.kt" to 6), findings(rule, one("$header\nfun bare(c: Context) {\n    Toast.makeText(c, \"y\", 0)\n}\n")))
        // (B.kt's own initializer toast is never shown in B and is reported there, in either case.)
        assertEquals(
            emptyList(),
            findings(rule, mapOf("B.kt" to "$header$property", "A.kt" to "$header\n$use")).filter { it.first == "A.kt" },
        )
    }

    // GetSignatures: flags held by a val.
    @Test fun getSignatures() {
        val rule = "GetSignatures"
        val header = "package p\nimport android.content.pm.PackageManager\n"
        val use = "fun f(pm: PackageManager) { pm.getPackageInfo(\"x\", SIG) }\n"
        val sameFile = "val SIG = PackageManager.GET_SIGNATURES\n"
        assertEquals(listOf("All.kt" to 4), findings(rule, one("$header$sameFile$use")))
        assertEquals(
            emptyList(),
            findings(rule, mapOf("B.kt" to "$header$sameFile", "A.kt" to "$header$use")),
        )
        // A const keeps its value in a class file, also one that is an expression.
        for (value in listOf("64", "0x40 or 1", "1 shl 6")) {
            assertEquals(
                listOf("A.kt" to 3),
                findings(rule, mapOf("B.kt" to "package p\nconst val SIG = $value\n", "A.kt" to "$header$use")),
                "const val SIG = $value",
            )
        }
    }

    // GlobalScopeLaunchInViewModel: a scope held by a val.
    @Test fun globalScopeLaunchInViewModel() {
        val rule = "GlobalScopeLaunchInViewModel"
        val header = "package p\nimport kotlinx.coroutines.CoroutineScope\nimport kotlinx.coroutines.GlobalScope\nimport kotlinx.coroutines.launch\n"
        val scope = "val appScope: CoroutineScope = GlobalScope\n"
        val use = "class FooViewModel { fun f() { appScope.launch { } } }\n"
        assertEquals(listOf("All.kt" to 6), findings(rule, one("$header$scope$use")))
        assertEquals(
            emptyList(),
            findings(
                rule,
                mapOf(
                    "B.kt" to "package p\nimport kotlinx.coroutines.CoroutineScope\nimport kotlinx.coroutines.GlobalScope\n$scope",
                    "A.kt" to "package p\nimport kotlinx.coroutines.launch\n$use",
                ),
            ),
        )
        // A receiver spelled GlobalScope still fires without any read.
        assertEquals(
            listOf("All.kt" to 6),
            findings(rule, one("$header\nclass BarViewModel { fun f() { GlobalScope.launch { } } }\n")),
        )
    }

    // TooGenericExceptionThrown: a configured name matches a library class, never a Kotlin class of this module,
    // whichever file it is declared in.
    @Test fun tooGenericExceptionThrown() {
        val rule = "TooGenericExceptionThrown"
        val options = mapOf<String, Any?>(
            "exceptionNames" to listOf("Exception", "Throwable", "Error", "RuntimeException", "CustomException", "IOException"),
        )
        val custom = "class CustomException : RuntimeException()\n"
        val use = "fun f() { throw CustomException() }\n"
        assertEquals(emptyList(), findings(rule, one("package p\n$custom$use"), options))
        assertEquals(
            emptyList(),
            findings(rule, mapOf("B.kt" to "package p\n$custom", "A.kt" to "package p\n$use"), options),
        )
        // A library (Java) class of a configured name is still generic.
        assertEquals(
            listOf("All.kt" to 2),
            findings(rule, one("package p\nfun f() { throw java.io.IOException() }\n"), options),
        )
    }
}
