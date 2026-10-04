// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): flags that ask for GET_SIGNATURES which Go misses. Go
// matches the spelling GET_SIGNATURES or a decimal, hex, or long literal inside
// the argument, and follows only a local named by the whole argument, through
// its initializer, inside a named function.
package test

import android.content.pm.PackageManager
import android.content.pm.PackageManager.GET_SIGNATURES as SIGNATURES

fun importAlias(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", SIGNATURES)<!>

fun binaryLiteral(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", 0b1000000)<!>

fun localInExpression(pm: PackageManager) {
    val base = PackageManager.GET_SIGNATURES
    <!GetSignatures!>pm.getPackageInfo("com.example", base or PackageManager.GET_META_DATA)<!>
}

fun localChain(pm: PackageManager) {
    val first = PackageManager.GET_SIGNATURES
    val second = first
    <!GetSignatures!>pm.getPackageInfo("com.example", second)<!>
}

fun assignedVar(pm: PackageManager) {
    var flags = 0
    flags = flags or PackageManager.GET_SIGNATURES
    <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
}

fun compoundAssignedVar(pm: PackageManager) {
    var flags = PackageManager.GET_META_DATA
    flags += 64
    <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
}

// The flags follow a named argument in their own position; Go counts only
// unlabeled arguments when it looks for the second one.
class PackageRepository {
    fun getPackageInfo(name: String, flags: Int): String = name + flags
}

fun namedThenPositional(repository: PackageRepository) =
    <!GetSignatures!>repository.getPackageInfo(name = "com.example", PackageManager.GET_SIGNATURES)<!>

private const val LEGACY_FLAGS = PackageManager.GET_SIGNATURES

fun topLevelConstant(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", LEGACY_FLAGS)<!>

class Member(private val pm: PackageManager) {
    private val flags = PackageManager.GET_SIGNATURES

    fun use() = <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>

    init {
        val local = PackageManager.GET_SIGNATURES
        println(<!GetSignatures!>pm.getPackageInfo("com.example", local)<!>)
    }

    // Go needs an enclosing function to follow a local.
    val lazyInfo = run {
        var bits = 0
        bits = bits or PackageManager.GET_SIGNATURES
        <!GetSignatures!>pm.getPackageInfo("com.example", bits)<!>
    }
}

// -1 sets every bit, GET_SIGNATURES included; Go reads the digits as 1.
fun allBits(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", -1)<!>

// The constant flags add up to 0x40; Go sees only literals without the bit.
fun foldedConstant(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", 0x20 + 0x20)<!>

// Every flag but GET_META_DATA, GET_SIGNATURES included.
fun invertedOtherFlag(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_META_DATA.inv())<!>

// The copy is taken before the reassignment; Go looks up only a local
// initialized with GET_SIGNATURES itself.
fun copiedBeforeReassignment(pm: PackageManager) {
    var flags = PackageManager.GET_SIGNATURES
    val copy = flags
    flags = 0
    println(flags)
    <!GetSignatures!>pm.getPackageInfo("com.example", copy)<!>
}

// The loop's later iterations read the flag assigned at the end of the body;
// Go reads the initializer only.
fun assignedLaterInLoop(pm: PackageManager, names: List<String>) {
    var flags = 0
    for (name in names) {
        <!GetSignatures!>pm.getPackageInfo(name, flags)<!>
        flags = PackageManager.GET_SIGNATURES
    }
}

// Go takes the second unlabeled argument (the user id) and never looks at the
// argument labeled `flags` once it has one.
class ProfileRepository {
    fun getPackageInfo(name: String, userId: Int, flags: Int): String = name + userId + flags
}

fun flagsAfterPositionalUserId(repository: ProfileRepository) =
    <!GetSignatures!>repository.getPackageInfo("com.example", 0, flags = PackageManager.GET_SIGNATURES)<!>

// Go's local lookup stops at the nearest function, the anonymous object's run.
fun capturedByObject(pm: PackageManager): Runnable {
    val flags = PackageManager.GET_SIGNATURES
    return object : Runnable {
        override fun run() {
            <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
        }
    }
}

// An anonymous object's own member.
fun objectMember(pm: PackageManager) = object {
    val flags = PackageManager.GET_SIGNATURES

    fun query() = <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
}

// Go follows a local only when it is the whole argument.
fun localInPackageInfoFlags(pm: PackageManager) {
    val flags = PackageManager.GET_SIGNATURES
    <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.PackageInfoFlags.of(flags.toLong()))<!>
}

// Go reads only a local var's initializer; the lambda sets the flag before the
// call.
fun assignedInLambda(pm: PackageManager) {
    var flags = 0
    run { flags = PackageManager.GET_SIGNATURES }
    <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
}

// Go reads the call's name from a plain identifier; a parenthesized callee
// has none.
fun parenthesizedInvoke(getPackageInfo: (String, Int) -> Unit) {
    <!GetSignatures!>(getPackageInfo)("com.example", PackageManager.GET_SIGNATURES)<!>
}

// Go reads the value arguments only; here the flags come from a trailing
// lambda.
class LazyFlagsRepository {
    fun getPackageInfo(name: String, flags: () -> Int): String = name + flags()
}

fun trailingLambdaFlags(repository: LazyFlagsRepository) =
    <!GetSignatures!>repository.getPackageInfo("com.example") { PackageManager.GET_SIGNATURES }<!>
