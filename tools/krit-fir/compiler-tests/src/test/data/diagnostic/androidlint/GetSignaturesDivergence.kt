// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 16, 25, 36, 45
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
