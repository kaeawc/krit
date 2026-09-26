// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 14, 17, 21, 22, 23, 24, 30, 38, 45, 54, 64, 72, 78, 79, 83, 87, 93, 98, 102, 107, 111
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
    }

    fun localVal(pm: PackageManager) {
        val flags = PackageManager.GET_SIGNATURES
        <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
    }

    // A reassignment that may not run leaves the initializer's flags.
    fun localVar(pm: PackageManager, modern: Boolean) {
        var flags = PackageManager.GET_SIGNATURES
        if (modern) flags = 0
        println(flags)
        <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
    }

    // A reassignment that keeps the flag.
    fun localVarKept(pm: PackageManager) {
        var flags = PackageManager.GET_SIGNATURES
        flags = flags or PackageManager.GET_META_DATA
        <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
    }

    // The reassignment is in the other branch.
    fun localVarOtherBranch(pm: PackageManager, modern: Boolean) {
        var flags = PackageManager.GET_SIGNATURES
        if (modern) {
            flags = 0
        } else {
            <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
        }
    }

    // The try block may throw before its reassignment.
    fun localVarFinally(pm: PackageManager) {
        var flags = PackageManager.GET_SIGNATURES
        try {
            flags = 0
        } finally {
            <!GetSignatures!>pm.getPackageInfo("com.example", flags)<!>
        }
    }

    // In a loop, the initializer reaches the first iteration.
    fun localVarInLoop(pm: PackageManager, names: List<String>) {
        var flags = PackageManager.GET_SIGNATURES
        for (name in names) {
            <!GetSignatures!>pm.getPackageInfo(name, flags)<!>
            flags = 0
        }
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
