package dev.jasonpearson.krit.fir.incremental

/**
 * Two-file scenarios for the split differential. The single-file split never separates a member or top-level
 * property from the code that uses it, so it cannot see rules that follow a use in file A to a declaration in
 * file B. Here B is the declaration and A the use; each file is compiled alone against the snapshot of the other.
 *
 * [differs] scenarios are cases where [rule] is known to give a different verdict on a slice, so the rule would have
 * to be tiered (or, for the withheld-facts scenario, the harness removes what keeps it equal). The others are safe controls: [rule] must give the same verdict, tiered or not. The rules that read other
 * files' bodies were narrowed to the file under check and to what class files keep (#873), so their scenarios are
 * controls; the depth chains below (A uses, B forwards, C holds the fact) are the research's probes, which needed
 * every file of the chain in source before the narrowing. SerialVersionUIDInSerializableClass is served from the
 * recorded class-name facts (#876): its control must not differ, and the scenario that withholds the facts must.
 * No built-in rule is tiered, so the withheld-facts scenario is the only [differs] one.
 */
internal class CrossFileScenario(
    val name: String,
    val rule: String,
    val files: Map<String, String>,
    val differs: Boolean,
    val config: Map<String, Map<String, Any?>> = emptyMap(),
    /** The slice is given no external class-name facts: a rule that needs them must then differ ([differs] = true). */
    val withholdExternalFacts: Boolean = false,
)

private const val TOAST_HEADER = "package p\nimport android.widget.Toast\nimport android.content.Context\n"
private const val GET_SIGNATURES_USE =
    "package p\nimport android.content.pm.PackageManager\nfun f(pm: PackageManager) { pm.getPackageInfo(\"x\", %s) }\n"

private val SERIAL_FILES = mapOf(
    "Resource.kt" to "package model\nsealed class Resource : java.io.Serializable {\n    companion object { private const val serialVersionUID = 1L }\n    data class Error(val message: String) : Resource()\n}\n",
    "Failures.kt" to "package app\nclass FatalFailure(msg: String) : kotlin.Error(msg)\nclass Plain(msg: String) : RuntimeException(msg)\n",
)

private fun signatureUse(flags: String) = GET_SIGNATURES_USE.format(flags)

internal val crossFileScenarios = listOf(
    CrossFileScenario(
        "SecureRandom control: top-level Random-typed val holding a SecureRandom in B (not followed)", "SecureRandom",
        mapOf(
            "B.kt" to "package p\nimport java.security.SecureRandom\nimport java.util.Random\nval rng: Random = SecureRandom()\n",
            "A.kt" to "package p\nfun f() { rng.setSeed(1L) }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "ShowToast control: var with a non-null initializer in B (followed as a class file has it), assigned and shown in A", "ShowToast",
        mapOf(
            "B.kt" to TOAST_HEADER + "lateinit var ctx: Context\nvar saved: Toast? = Toast.makeText(ctx, \"x\", 0)\n",
            "A.kt" to TOAST_HEADER + "fun g(c: Context) {\n    saved = Toast.makeText(c, \"y\", 0)\n    saved?.show()\n}\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "ShowToast control: var = null in B (a literal null initializer)", "ShowToast",
        mapOf(
            "B.kt" to TOAST_HEADER + "var saved: Toast? = null\n",
            "A.kt" to TOAST_HEADER + "fun g(c: Context) {\n    saved = Toast.makeText(c, \"y\", 0)\n    saved?.show()\n}\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "GetSignatures control: non-const val holding GET_SIGNATURES in B", "GetSignatures",
        mapOf(
            "B.kt" to "package p\nimport android.content.pm.PackageManager\nval SIG_FLAGS = PackageManager.GET_SIGNATURES\n",
            "A.kt" to signatureUse("SIG_FLAGS"),
        ),
        differs = false,
    ),
    CrossFileScenario(
        "GetSignatures control: non-const val SIG = 64 in B", "GetSignatures",
        mapOf("B.kt" to "package p\nval SIG = 64\n", "A.kt" to signatureUse("SIG")),
        differs = false,
    ),
    CrossFileScenario(
        "GetSignatures control: lookalike non-const GET_SIGNATURES = 0 in an object in B", "GetSignatures",
        mapOf(
            "B.kt" to "package p\nobject Flags { val GET_SIGNATURES = 0 }\n",
            "A.kt" to signatureUse("Flags.GET_SIGNATURES"),
        ),
        differs = false,
    ),
    CrossFileScenario(
        "GetSignatures control: const val SIG = 64 in B", "GetSignatures",
        mapOf("B.kt" to "package p\nconst val SIG = 64\n", "A.kt" to signatureUse("SIG")),
        differs = false,
    ),
    CrossFileScenario(
        "GetSignatures control: const val SIG = 0x40 or 1 in B", "GetSignatures",
        mapOf("B.kt" to "package p\nconst val SIG = 0x40 or 1\n", "A.kt" to signatureUse("SIG")),
        differs = false,
    ),
    CrossFileScenario(
        "GetSignatures control: lookalike const GET_SIGNATURES = 0 in B", "GetSignatures",
        mapOf(
            "B.kt" to "package p\nobject Flags { const val GET_SIGNATURES = 0 }\n",
            "A.kt" to signatureUse("Flags.GET_SIGNATURES"),
        ),
        differs = false,
    ),
    CrossFileScenario(
        "GlobalScopeLaunchInViewModel control: val appScope: CoroutineScope = GlobalScope in B", "GlobalScopeLaunchInViewModel",
        mapOf(
            "B.kt" to "package p\nimport kotlinx.coroutines.CoroutineScope\nimport kotlinx.coroutines.GlobalScope\nval appScope: CoroutineScope = GlobalScope\n",
            "A.kt" to "package p\nimport kotlinx.coroutines.launch\nclass FooViewModel { fun f() { appScope.launch { } } }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "GlobalScopeLaunchInViewModel control: lookalike val GlobalScope = MainScope() in B", "GlobalScopeLaunchInViewModel",
        mapOf(
            "B.kt" to "package p\nimport kotlinx.coroutines.CoroutineScope\nimport kotlinx.coroutines.MainScope\nval GlobalScope: CoroutineScope = MainScope()\n",
            "A.kt" to "package p\nimport kotlinx.coroutines.launch\nclass FooViewModel { fun f() { GlobalScope.launch { } } }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "TooGenericExceptionThrown control: configured name matches a project class in B", "TooGenericExceptionThrown",
        mapOf(
            "B.kt" to "package p\nclass CustomException : RuntimeException()\n",
            "A.kt" to "package p\nfun f() { throw CustomException() }\n",
        ),
        differs = false,
        config = mapOf(
            "TooGenericExceptionThrown" to mapOf(
                "exceptionNames" to listOf("Exception", "Throwable", "Error", "RuntimeException", "CustomException"),
            ),
        ),
    ),
    CrossFileScenario(
        "TooGenericExceptionThrown control: default config, project class named Exception in B", "TooGenericExceptionThrown",
        mapOf(
            "B.kt" to "package p\nclass Exception : RuntimeException()\n",
            "A.kt" to "package p\nfun f() { throw Exception() }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "BindsReturnTypeMatchesParam control: qualifier with a default argument in B, differing argument in A",
        "BindsReturnTypeMatchesParam",
        mapOf(
            "B.kt" to "package p\nimport javax.inject.Qualifier\n@Qualifier\nannotation class Lvl(val level: Int = 1)\ninterface Foo\n",
            "A.kt" to "package p\nimport dagger.Binds\nimport dagger.Module\n@Module\nabstract class M {\n    @Binds\n    @Lvl\n    abstract fun f(@Lvl(level = 2) foo: Foo): Foo\n}\n",
        ),
        differs = false,
    ),
    // SerialVersionUIDInSerializableClass is not tiered: a slice gets the recorded class-name facts of the files it
    // reads as binaries. Failures.kt reaches Serializable only through Resource.Error in the binary Resource.kt.
    CrossFileScenario(
        "SerialVersionUIDInSerializableClass control: Error reaches Serializable through Resource in B",
        "SerialVersionUIDInSerializableClass",
        SERIAL_FILES,
        differs = false,
    ),
    // The same module with the external facts withheld must differ, so the control above cannot pass vacuously.
    CrossFileScenario(
        "SerialVersionUIDInSerializableClass: external class-name facts withheld", "SerialVersionUIDInSerializableClass",
        SERIAL_FILES,
        differs = true, withholdExternalFacts = true,
    ),
    // Regression for the Wakelock offset coincidence: B's top-level `lock` is declared at source offsets that fall
    // inside A's function, so a source-offset-only locality check took it for a local of that function.
    CrossFileScenario(
        "Wakelock control: top-level val in B whose offsets fall inside A's function", "Wakelock",
        mapOf(
            "B.kt" to "package p\nclass Locker { fun newWakeLock(): Locker = this; fun acquire() {}; fun release() {} }\nval lock = Locker().newWakeLock()\n",
            "A.kt" to "package p\nfun f() {\n" + "    // " + "x".repeat(60) + "\n" + "    // " + "x".repeat(60) + "\n" +
                "    // " + "x".repeat(60) + "\n" + "    // " + "x".repeat(60) + "\n    lock.acquire()\n" +
                "    // " + "x".repeat(60) + "\n    // " + "x".repeat(60) + "\n}\n",
        ),
        differs = false,
    ),
    // Consts whose class-file value differs from the source expression: a class file keeps the evaluated value,
    // so a rule must read an evaluated const of another file the same way on a slice and on a full compile.
    CrossFileScenario(
        "GetSignatures control: const val SIG = 1 shl 6 in B (evaluated value)", "GetSignatures",
        mapOf("B.kt" to "package p\nconst val SIG = 1 shl 6\n", "A.kt" to signatureUse("SIG")),
        differs = false,
    ),
    CrossFileScenario(
        "ShortAlarm control: const val EVERY = 5 * 1000L in B (evaluated value)", "ShortAlarm",
        mapOf(
            "B.kt" to "package p\nconst val EVERY = 5 * 1000L\n",
            "A.kt" to "package p\nimport android.app.AlarmManager\nimport android.app.PendingIntent\nfun f(am: AlarmManager, pi: PendingIntent) { am.setRepeating(0, 0L, EVERY, pi) }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "ShortAlarm control: wrapper declared in B ignores its interval (no body is read across files)", "ShortAlarm",
        mapOf(
            "B.kt" to "package p\nimport android.app.PendingIntent\nobject Sched { fun setRepeating(t: Int, at: Long, every: Long, pi: PendingIntent) { println(t) } }\n",
            "A.kt" to "package p\nimport android.app.PendingIntent\nfun f(pi: PendingIntent) { Sched.setRepeating(0, 0L, 5000L, pi) }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "WorldWriteableFiles control: const val MODE_WORLD_WRITEABLE = 1 shl 1 in B", "WorldWriteableFiles",
        mapOf(
            "B.kt" to "package p\nconst val MODE_WORLD_WRITEABLE = 1 shl 1\n",
            "A.kt" to "package p\nfun mode() = MODE_WORLD_WRITEABLE\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "WorldWriteableFiles control: non-const val MODE_WORLD_WRITEABLE = 0 in B (unknown, reported as Go does)", "WorldWriteableFiles",
        mapOf(
            "B.kt" to "package p\nobject B { val MODE_WORLD_WRITEABLE = 0 }\n",
            "A.kt" to "package p\nfun mode() = B.MODE_WORLD_WRITEABLE\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "JdbcStatementExecute control: non-const val in B (unknown across files, quiet)", "JdbcStatementExecute",
        mapOf(
            "B.kt" to "package p\nval QUERY: String = \"SELECT 1\"\n",
            "A.kt" to "package p\nimport java.sql.Statement\nfun run(s: Statement) { s.execute(QUERY) }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "JdbcStatementExecute control: object member val in B plus a literal (unknown across files, quiet)", "JdbcStatementExecute",
        mapOf(
            "B.kt" to "package p\nobject Queries { val selectAll = \"SELECT * FROM t\" }\n",
            "A.kt" to "package p\nimport java.sql.Statement\nfun run(s: Statement) { s.execute(Queries.selectAll + \" LIMIT 1\") }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "ShortAlarm control: unrelated Poller.setRepeating in B (signature is not an alarm API)", "ShortAlarm",
        mapOf(
            "B.kt" to "package p\nclass Poller { fun setRepeating(name: String, delay: Long, intervalMs: Long, task: Runnable) {} }\n",
            "A.kt" to "package p\nfun f(x: Poller, r: Runnable) { x.setRepeating(\"n\", 0L, 5000L, r) }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "GetSignatures control: project val GET_SIGNATURES = 0 in B (lookalike, quiet)", "GetSignatures",
        mapOf("B.kt" to "package p\nval GET_SIGNATURES = 0\n", "A.kt" to signatureUse("GET_SIGNATURES")),
        differs = false,
    ),
    CrossFileScenario(
        "JdbcStatementExecute control: const val query in B (static across files)", "JdbcStatementExecute",
        mapOf(
            "B.kt" to "package p\nconst val QUERY = \"SELECT 1\"\n",
            "A.kt" to "package p\nimport java.sql.Statement\nfun run(s: Statement) { s.execute(QUERY) }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "TooGenericExceptionThrown control: configured name matches a Java class (reported either way)", "TooGenericExceptionThrown",
        mapOf(
            "A.kt" to "package p\nfun f() { throw java.io.IOException() }\n",
        ),
        differs = false,
        config = mapOf(
            "TooGenericExceptionThrown" to mapOf(
                "exceptionNames" to listOf("Exception", "Throwable", "Error", "RuntimeException", "IOException"),
            ),
        ),
    ),
    CrossFileScenario(
        "HardcodedSecretKey chain over 4 files (A uses, the last holds the fact)", "HardcodedSecretKey",
        mapOf(
            "A.kt" to "package p\nimport javax.crypto.spec.SecretKeySpec\nfun key() = SecretKeySpec(java.util.Base64.getDecoder().decode(B.K1), \"AES\")\n",
            "B.kt" to "package p\nobject B { val K1: String = C.K2 }\n",
            "C.kt" to "package p\nobject C { val K2: String = D.K3 }\n",
            "D.kt" to "package p\nobject D { val K3: String = \"c2VjcmV0a2V5MTIzNDU2Nw==\" }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "StaticIv chain over 3 files (A uses, the last holds the fact)", "StaticIv",
        mapOf(
            "A.kt" to "package p\nimport javax.crypto.spec.IvParameterSpec\nfun iv() = IvParameterSpec(\"\${B.P}\".toByteArray())\n",
            "B.kt" to "package p\nobject B { val P: String = \"\${C.Q}\" }\n",
            "C.kt" to "package p\nobject C { val Q: String = \"0123456789abcdef\" }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "WeakKeySize chain over 2 files (A uses, the last holds the fact)", "WeakKeySize",
        mapOf(
            "A.kt" to "package p\nfun weak() { gen.init(64) }\n",
            "B.kt" to "package p\nimport javax.crypto.KeyGenerator\nval gen: KeyGenerator = KeyGenerator.getInstance(\"AES\")\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "LongLogTag chain over 3 files (A uses, the last holds the fact)", "LongLogTag",
        mapOf(
            "A.kt" to "package p\nimport android.util.Log\nfun log() { Log.d(B.TAG, \"m\") }\n",
            "B.kt" to "package p\nobject B { val TAG: String = C.TAG }\n",
            "C.kt" to "package p\nobject C { val TAG: String = \"AVeryLongTagThatExceedsTheLimit\" }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "LongLogTag: const tag with an escape sequence in B (the message spells the decoded value)", "LongLogTag",
        mapOf(
            "A.kt" to "package p\nimport android.util.Log\nfun log() { Log.d(Tags.T, \"m\") }\n",
            "B.kt" to "package p\nobject Tags { const val T = \"Tag\\u0041VeryLongTagThatExceedsTheLimit\" }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "LocaleGetDefaultForFormatting chain over 3 files (A uses, the last holds the fact)", "LocaleGetDefaultForFormatting",
        mapOf(
            "A.kt" to "package p\nimport java.util.Locale\nfun fmt() = F1.withLocale(Locale.getDefault())\n",
            "B.kt" to "package p\nimport java.time.format.DateTimeFormatter\nval F1: DateTimeFormatter = F0\n",
            "C.kt" to "package p\nimport java.time.format.DateTimeFormatter\nval F0: DateTimeFormatter = DateTimeFormatter.ISO_DATE\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "JdbcStatementExecute chain over 3 files (A uses, the last holds the fact)", "JdbcStatementExecute",
        mapOf(
            "A.kt" to "package p\nimport java.sql.Statement\nfun run(s: Statement) { s.execute(B.query) }\n",
            "B.kt" to "package p\nobject B { val query: String = C.base }\n",
            "C.kt" to "package p\nobject C { val base: String = \"SELECT 1\" }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "CollectionsSynchronizedListIteration chain over 2 files (A uses, the last holds the fact)", "CollectionsSynchronizedListIteration",
        mapOf(
            "A.kt" to "package p\nfun dump() { for (x in items) println(x) }\n",
            "B.kt" to "package p\nimport java.util.Collections\nval items: MutableList<String> = Collections.synchronizedList(ArrayList())\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "SharedFlowWithoutReplay chain over 3 files (A uses, the last holds the fact)", "SharedFlowWithoutReplay",
        mapOf(
            "A.kt" to "package p\nclass Vm { val events = factory1<Int>() }\n",
            "B.kt" to "package p\nimport kotlinx.coroutines.flow.MutableSharedFlow\nfun <T> factory1(): MutableSharedFlow<T> = factory2()\n",
            "C.kt" to "package p\nimport kotlinx.coroutines.flow.MutableSharedFlow\nfun <T> factory2(): MutableSharedFlow<T> = MutableSharedFlow()\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "ShortAlarm chain over 3 files (A uses, the last holds the fact)", "ShortAlarm",
        mapOf(
            "A.kt" to "package p\nimport android.app.PendingIntent\nfun schedule(pi: PendingIntent) { Sched.setRepeating(0, 0L, 5000L, pi) }\n",
            "B.kt" to "package p\nimport android.app.PendingIntent\nobject Sched { fun setRepeating(t: Int, at: Long, every: Long, pi: PendingIntent) { Inner.setRepeating(t, at, every, pi) } }\n",
            "C.kt" to "package p\nimport android.app.AlarmManager\nimport android.app.PendingIntent\nobject Inner {\n  private fun am(): AlarmManager = TODO()\n  fun setRepeating(t: Int, at: Long, every: Long, pi: PendingIntent) { println(t) }\n}\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "WorldWriteableFiles chain over 3 files (A uses, the last holds the fact)", "WorldWriteableFiles",
        mapOf(
            "A.kt" to "package p\nfun mode() = B.MODE_WORLD_WRITEABLE\n",
            "B.kt" to "package p\nobject B { val MODE_WORLD_WRITEABLE: Int = C.MODE }\n",
            "C.kt" to "package p\nobject C { val MODE: Int = 0 }\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "SecureRandom chain over 2 files (A uses, the last holds the fact)", "SecureRandom",
        mapOf(
            "A.kt" to "package p\nfun seed() { rng.setSeed(1L) }\n",
            "B.kt" to "package p\nimport java.security.SecureRandom\nimport java.util.Random\nval rng: Random = SecureRandom()\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "ShowToast chain over 2 files (A uses, the last holds the fact)", "ShowToast",
        mapOf(
            "A.kt" to "package p\nimport android.widget.Toast\nimport android.content.Context\nfun g(c: Context) {\n    saved = Toast.makeText(c, \"y\", 0)\n    saved?.show()\n}\n",
            "B.kt" to "package p\nimport android.widget.Toast\nimport android.content.Context\nlateinit var ctx: Context\nvar saved: Toast? = Toast.makeText(ctx, \"x\", 0)\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "GetSignatures chain over 3 files (A uses, the last holds the fact)", "GetSignatures",
        mapOf(
            "A.kt" to "package p\nimport android.content.pm.PackageManager\nfun f(pm: PackageManager) { pm.getPackageInfo(\"x\", FLAGS) }\n",
            "B.kt" to "package p\nval FLAGS = SIG\n",
            "C.kt" to "package p\nimport android.content.pm.PackageManager\nval SIG = PackageManager.GET_SIGNATURES\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "GlobalScopeLaunchInViewModel chain over 3 files (A uses, the last holds the fact)", "GlobalScopeLaunchInViewModel",
        mapOf(
            "A.kt" to "package p\nimport kotlinx.coroutines.launch\nclass FooViewModel { fun f() { appScope.launch { } } }\n",
            "B.kt" to "package p\nimport kotlinx.coroutines.CoroutineScope\nval appScope: CoroutineScope = rootScope\n",
            "C.kt" to "package p\nimport kotlinx.coroutines.CoroutineScope\nimport kotlinx.coroutines.GlobalScope\nval rootScope: CoroutineScope = GlobalScope\n",
        ),
        differs = false,
    ),
    CrossFileScenario(
        "TooGenericExceptionThrown chain over 2 files (A uses, the last holds the fact)", "TooGenericExceptionThrown",
        mapOf(
            "A.kt" to "package p\nfun f() { throw CustomException() }\n",
            "B.kt" to "package p\nclass CustomException : RuntimeException()\n",
        ),
        differs = false,
        config = mapOf(
            "TooGenericExceptionThrown" to mapOf(
                "exceptionNames" to listOf("Exception", "Throwable", "Error", "RuntimeException", "CustomException"),
            ),
        ),
    ),
)
