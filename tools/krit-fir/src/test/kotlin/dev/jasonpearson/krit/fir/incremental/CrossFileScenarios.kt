package dev.jasonpearson.krit.fir.incremental

/**
 * Two-file scenarios for the split differential. The single-file split never separates a member or top-level
 * property from the code that uses it, so it cannot see rules that follow a use in file A to a declaration in
 * file B. Here B is the declaration and A the use; each file is compiled alone against the snapshot of the other.
 *
 * [differs] scenarios are cases where [rule] is known to give a different verdict on a slice, so the rule must be
 * tiered. The others are safe controls: [rule] must give the same verdict, tiered or not.
 */
internal class CrossFileScenario(
    val name: String,
    val rule: String,
    val files: Map<String, String>,
    val differs: Boolean,
    val config: Map<String, Map<String, Any?>> = emptyMap(),
)

private const val TOAST_HEADER = "package p\nimport android.widget.Toast\nimport android.content.Context\n"
private const val GET_SIGNATURES_USE =
    "package p\nimport android.content.pm.PackageManager\nfun f(pm: PackageManager) { pm.getPackageInfo(\"x\", %s) }\n"

private fun signatureUse(flags: String) = GET_SIGNATURES_USE.format(flags)

internal val crossFileScenarios = listOf(
    CrossFileScenario(
        "SecureRandom: top-level Random-typed val holding a SecureRandom", "SecureRandom",
        mapOf(
            "B.kt" to "package p\nimport java.security.SecureRandom\nimport java.util.Random\nval rng: Random = SecureRandom()\n",
            "A.kt" to "package p\nfun f() { rng.setSeed(1L) }\n",
        ),
        differs = true,
    ),
    CrossFileScenario(
        "ShowToast: var with a non-null initializer in B, assigned and shown in A", "ShowToast",
        mapOf(
            "B.kt" to TOAST_HEADER + "lateinit var ctx: Context\nvar saved: Toast? = Toast.makeText(ctx, \"x\", 0)\n",
            "A.kt" to TOAST_HEADER + "fun g(c: Context) {\n    saved = Toast.makeText(c, \"y\", 0)\n    saved?.show()\n}\n",
        ),
        differs = true,
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
        "GetSignatures: non-const val holding GET_SIGNATURES in B", "GetSignatures",
        mapOf(
            "B.kt" to "package p\nimport android.content.pm.PackageManager\nval SIG_FLAGS = PackageManager.GET_SIGNATURES\n",
            "A.kt" to signatureUse("SIG_FLAGS"),
        ),
        differs = true,
    ),
    CrossFileScenario(
        "GetSignatures: non-const val SIG = 64 in B", "GetSignatures",
        mapOf("B.kt" to "package p\nval SIG = 64\n", "A.kt" to signatureUse("SIG")),
        differs = true,
    ),
    CrossFileScenario(
        "GetSignatures: lookalike non-const GET_SIGNATURES = 0 in an object in B", "GetSignatures",
        mapOf(
            "B.kt" to "package p\nobject Flags { val GET_SIGNATURES = 0 }\n",
            "A.kt" to signatureUse("Flags.GET_SIGNATURES"),
        ),
        differs = true,
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
        "GlobalScopeLaunchInViewModel: val appScope: CoroutineScope = GlobalScope in B", "GlobalScopeLaunchInViewModel",
        mapOf(
            "B.kt" to "package p\nimport kotlinx.coroutines.CoroutineScope\nimport kotlinx.coroutines.GlobalScope\nval appScope: CoroutineScope = GlobalScope\n",
            "A.kt" to "package p\nimport kotlinx.coroutines.launch\nclass FooViewModel { fun f() { appScope.launch { } } }\n",
        ),
        differs = true,
    ),
    CrossFileScenario(
        "GlobalScopeLaunchInViewModel: lookalike val GlobalScope = MainScope() in B", "GlobalScopeLaunchInViewModel",
        mapOf(
            "B.kt" to "package p\nimport kotlinx.coroutines.CoroutineScope\nimport kotlinx.coroutines.MainScope\nval GlobalScope: CoroutineScope = MainScope()\n",
            "A.kt" to "package p\nimport kotlinx.coroutines.launch\nclass FooViewModel { fun f() { GlobalScope.launch { } } }\n",
        ),
        differs = true,
    ),
    CrossFileScenario(
        "TooGenericExceptionThrown: configured name matches a project class in B", "TooGenericExceptionThrown",
        mapOf(
            "B.kt" to "package p\nclass CustomException : RuntimeException()\n",
            "A.kt" to "package p\nfun f() { throw CustomException() }\n",
        ),
        differs = true,
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
)
