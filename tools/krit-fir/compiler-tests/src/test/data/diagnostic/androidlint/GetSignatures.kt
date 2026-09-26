// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 14, 17, 21, 22, 23, 24, 31, 39, 43, 44, 48, 52, 58, 63, 67, 72, 76
package test

import android.content.Context
import android.content.pm.PackageInfo
import android.content.pm.PackageManager
import android.content.pm.PackageManager.GET_SIGNATURES

class SignatureChecker(private val context: Context) {
    fun direct(pm: PackageManager): PackageInfo =
        <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>

    fun staticImport(pm: PackageManager): PackageInfo = <!GetSignatures!>pm.getPackageInfo("com.example", GET_SIGNATURES)<!>

    fun combined(pm: PackageManager) {
        <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_META_DATA or PackageManager.GET_SIGNATURES)<!>
    }

    fun literals(pm: PackageManager) {
        <!GetSignatures!>pm.getPackageInfo("com.example", 64)<!>
        <!GetSignatures!>pm.getPackageInfo("com.example", 0x40)<!>
        <!GetSignatures!>pm.getPackageInfo("com.example", 0x41L.toInt())<!>
        <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_META_DATA or 64)<!>
        pm.getPackageInfo("com.example", 128)
        pm.getPackageInfo("com.example", -1)
    }

    fun localVal(pm: PackageManager) {
        val flags = PackageManager.GET_SIGNATURES
        <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
    }

    // Like Go, a local var counts through its initializer even when it is
    // reassigned before the call.
    fun localVar(pm: PackageManager) {
        var flags = PackageManager.GET_SIGNATURES
        flags = 0
        <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
    }

    fun packageInfoFlags(pm: PackageManager) {
        <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.PackageInfoFlags.of(PackageManager.GET_SIGNATURES.toLong()))<!>
        <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.PackageInfoFlags.of(64L))<!>
    }

    fun conditional(pm: PackageManager, legacy: Boolean) {
        <!GetSignatures!>pm.getPackageInfo("com.example", if (legacy) PackageManager.GET_SIGNATURES else PackageManager.GET_SIGNING_CERTIFICATES)<!>
    }

    fun chain() {
        <!GetSignatures!>context<!>
            .packageManager
            .getPackageInfo(context.packageName, PackageManager.GET_SIGNATURES)
    }

    fun safeCall(pm: PackageManager?) {
        <!GetSignatures!>pm<!>
            ?.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
    }

    fun implicitReceiver(pm: PackageManager) = with(pm) {
        <!GetSignatures!>getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>
    }

    fun inLambda(pm: PackageManager, names: List<String>) = names.map {
        <!GetSignatures!>pm.getPackageInfo(it, PackageManager.GET_SIGNATURES)<!>
    }

    fun inAnonymousObject(pm: PackageManager): Runnable = object : Runnable {
        override fun run() {
            <!GetSignatures!>pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>
        }
    }

    val property: PackageInfo = <!GetSignatures!>context.packageManager.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)<!>

    fun modern(pm: PackageManager) {
        pm.getPackageInfo("com.example", PackageManager.GET_SIGNING_CERTIFICATES)
        pm.getPackageInfo("com.example", PackageManager.GET_META_DATA)
        pm.getPackageInfo("com.example", 0)
    }

    // The constant read outside the flags argument is not a query.
    fun incidental(pm: PackageManager) {
        val flags = PackageManager.GET_SIGNATURES
        println(flags)
        pm.getPackageInfo("com.example", 0)
    }

    // A parameter's default value is not followed.
    fun parameter(pm: PackageManager, flags: Int = PackageManager.GET_SIGNATURES) {
        pm.getPackageInfo("com.example", flags)
    }
}
