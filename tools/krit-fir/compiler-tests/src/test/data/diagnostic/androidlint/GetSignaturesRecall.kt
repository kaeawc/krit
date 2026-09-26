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
}
