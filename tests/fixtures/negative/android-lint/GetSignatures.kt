package com.example

import android.content.pm.PackageManager
import android.os.Build

private const val KNOWN_SIG = "abc"

class SignatureChecker {
    fun checkModern(pm: PackageManager) {
        val info = pm.getPackageInfo("com.example", PackageManager.GET_SIGNING_CERTIFICATES)
    }

    fun isTrustedPackage(pm: PackageManager, packageName: String): Boolean {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            val info = pm.getPackageInfo(packageName, PackageManager.GET_SIGNING_CERTIFICATES)
            info.signingInfo?.apkContentsSigners?.any { it.toCharsString() == KNOWN_SIG } == true
        } else {
            @Suppress("DEPRECATION")
            val info = pm.getPackageInfo(packageName, PackageManager.GET_SIGNATURES)
            info.signatures?.any { it.toCharsString() == KNOWN_SIG } == true
        }
    }
}
