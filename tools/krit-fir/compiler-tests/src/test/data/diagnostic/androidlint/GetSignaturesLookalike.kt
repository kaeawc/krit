// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 20, 21, 30, 39, 43
package test

import android.content.pm.PackageManager

// A project method named getPackageInfo still receives the deprecated flag,
// and Go matches the call by name: both report it.
class PackageRepository {
    fun getPackageInfo(name: String, flags: Int): String = name + flags
}

fun wrapper(repository: PackageRepository) =
    <!GetSignatures!>repository.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>

fun wrapperNeutral(repository: PackageRepository) = repository.getPackageInfo("com.example", 1)

// Named arguments: the one labeled `flags`, in any order.
fun wrapperNamed(repository: PackageRepository) {
    <!GetSignatures!>repository.getPackageInfo(name = "com.example", flags = PackageManager.GET_SIGNATURES)<!>
    <!GetSignatures!>repository.getPackageInfo(flags = PackageManager.GET_SIGNATURES, name = "com.example")<!>
    repository.getPackageInfo(flags = PackageManager.GET_META_DATA, name = "GET_SIGNATURES")
}

// A project constant that forwards the platform flag under the same name.
object Constants {
    const val GET_SIGNATURES = PackageManager.GET_SIGNATURES
}

fun forwardedConstant(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", Constants.GET_SIGNATURES)<!>

// Divergence (precision): a project constant named GET_SIGNATURES whose value
// is a literal without the 0x40 bit is not the platform flag. Go reports the
// call by the name; the flags do not ask for signatures, so FIR does not.
object LocalFlags {
    const val GET_SIGNATURES = 1
}

fun lookalikeConstant(pm: PackageManager) = pm.getPackageInfo("com.example", LocalFlags.GET_SIGNATURES)

// A function-typed value called getPackageInfo, as Go reads the call's name.
fun invoked(getPackageInfo: (String, Int) -> Unit) {
    <!GetSignatures!>getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>
    getPackageInfo("com.example", PackageManager.GET_META_DATA)
}

// A single-argument getPackageInfo has no flags argument.
fun getPackageInfo(name: String): String = name

fun oneArgument() = getPackageInfo("com.example")
