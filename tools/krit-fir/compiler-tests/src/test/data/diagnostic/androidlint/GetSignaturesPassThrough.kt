// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 14, 16, 19, 21, 23, 26, 29, 31, 35, 39, 44, 52, 63
// The flags value reaches getPackageInfo through a lambda's result, a Kotlin
// library call that returns its receiver, an operand, or an element, a named
// `flags` argument after another named one, or a vararg's second element. Go
// reports each (the literal or the constant sits in the argument), and the
// value does carry the 0x40 bit.
package test

import android.content.pm.PackageManager

fun runResult(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", run { 64 })<!>

fun letResult(pm: PackageManager, flags: Int) = <!GetSignatures!>pm.getPackageInfo("com.example", flags.let { it or 64 })<!>

fun withResult(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", with(pm) { 64 })<!>

fun labeledReturn(pm: PackageManager, legacy: Boolean) =
    <!GetSignatures!>pm.getPackageInfo("com.example", run { if (legacy) return@run 64; 0 })<!>

fun alsoReceiver(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", 64.also { println(it) })<!>

fun applyReceiver(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", 64.apply { println(this) })<!>

fun takeIfReceiver(pm: PackageManager, legacy: Boolean) =
    <!GetSignatures!>pm.getPackageInfo("com.example", 64.takeIf { legacy } ?: 0)<!>

fun coerced(pm: PackageManager) =
    <!GetSignatures!>pm.getPackageInfo("com.example", (PackageManager.GET_META_DATA or 64).coerceAtLeast(0))<!>

fun maxOfOperand(pm: PackageManager, other: Int) = <!GetSignatures!>pm.getPackageInfo("com.example", maxOf(64, other))<!>

fun localFromRun(pm: PackageManager) {
    val flags = run { 64 }
    <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
}

// A Kotlin library call may hand back the literal it is given.
fun libraryElement(pm: PackageManager) = <!GetSignatures!>pm.getPackageInfo("com.example", listOf(64).first())<!>

fun libraryNeutral(pm: PackageManager, names: List<String>) =
    pm.getPackageInfo("com.example", names.indexOf("com.example").coerceAtLeast(0))

fun negativeLiteral(pm: PackageManager) =<!GetSignatures!>pm.getPackageInfo("com.example", -64)<!>

// Go takes the second unlabeled argument, or else the one labeled `flags`.
class UserRepository {
    fun getPackageInfo(name: String, userId: Int, flags: Int): String = name + userId + flags
}

fun namedFlagsAfterUserId(repository: UserRepository) =
    <!GetSignatures!>repository.getPackageInfo("com.example", userId = 0, flags = PackageManager.GET_SIGNATURES)<!>

fun namedUserIdOnly(repository: UserRepository) =
    repository.getPackageInfo("com.example", userId = 0, flags = PackageManager.GET_META_DATA)

// A vararg wrapper: the second argument is the flags, as Go reads it.
class VarargRepository {
    fun getPackageInfo(vararg args: Any): String = args.joinToString()
}

fun varargWrapper(repository: VarargRepository) =
    <!GetSignatures!>repository.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>

fun varargWrapperNeutral(repository: VarargRepository) =
    repository.getPackageInfo("com.example", PackageManager.GET_META_DATA)
