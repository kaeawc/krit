// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 16, 18, 21, 30, 41, 50, 56, 59, 62, 67, 68, 71, 78, 85
// Divergence (precision): Go findings whose flags do not ask for
// GET_SIGNATURES.
package test

import android.content.pm.PackageManager

// Go counts any integer literal inside the argument with the 0x40 bit; here
// 100 is compared, and 64 is an argument of an unrelated call, not the flags
// value.
fun comparedLiteral(pm: PackageManager, size: Int) = pm.getPackageInfo("com.example", if (size > 100) 0 else 1)

fun compute(value: Int): Int = value / 2

fun unrelatedCall(pm: PackageManager) = pm.getPackageInfo("com.example", compute(64))

fun unrelatedCallOfLambda(pm: PackageManager) = pm.getPackageInfo("com.example", compute(run { 64 }))

// The lambda prints 64 and returns 0, the flags.
fun lambdaStatement(pm: PackageManager) = pm.getPackageInfo("com.example", run { println(64); 0 })

// Go looks up a local named like the argument's last segment, so `config.flags`
// borrows the local `flags`; the config's flags are 0.
class Config(val flags: Int = 0)

fun receiverProperty(pm: PackageManager, config: Config) {
    val flags = PackageManager.GET_SIGNATURES
    println(flags)
    pm.getPackageInfo("com.example", config.flags)
}

// Go takes the first earlier local of that name in the function, in any
// scope; the call reads the later `flags`, which is GET_META_DATA.
fun otherScope(pm: PackageManager) {
    run {
        val flags = PackageManager.GET_SIGNATURES
        println(flags)
    }
    val flags = PackageManager.GET_META_DATA
    pm.getPackageInfo("com.example", flags)
}

// A local in a nested lambda does not reach the outer call either.
fun shadowedParameter(pm: PackageManager, flags: Int) {
    listOf(1).forEach {
        val flags = PackageManager.GET_SIGNATURES
        println(flags + it)
    }
    pm.getPackageInfo("com.example", flags)
}

// Go reports GET_SIGNATURES wherever it is read; `and GET_SIGNATURES.inv()`
// masks the flag out, so the flags never ask for it.
fun maskedOut(pm: PackageManager, flags: Int) =
    pm.getPackageInfo("com.example", flags and PackageManager.GET_SIGNATURES.inv())

fun maskedOutInLambda(pm: PackageManager, flags: Int?) =
    pm.getPackageInfo("com.example", (flags ?: 0).let { it and PackageManager.GET_SIGNATURES.inv() })

// A constant mask of 0 clears every flag.
fun maskedToZero(pm: PackageManager) = pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES and 0)

// Go judges a literal by its digits; as Int values, -65 and -100 have the 0x40
// bit clear, and `96 and 0x3F` is 0x20.
fun negativeWithoutBit(pm: PackageManager) {
    pm.getPackageInfo("com.example", -65)
    pm.getPackageInfo("com.example", -100)
}

fun maskedLiteral(pm: PackageManager) = pm.getPackageInfo("com.example", 96 and 0x3F)

// Go reads a local var's initializer only; a reassignment on every path to the
// call replaces GET_SIGNATURES before the call reads the flags.
fun reassignedVar(pm: PackageManager) {
    var flags = PackageManager.GET_SIGNATURES
    flags = 0
    pm.getPackageInfo("com.example", flags)
}

fun replacedVar(pm: PackageManager) {
    var flags = PackageManager.GET_SIGNATURES
    println(flags)
    flags = PackageManager.GET_SIGNING_CERTIFICATES
    pm.getPackageInfo("com.example", flags)
}
