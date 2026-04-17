package com.example

import android.content.pm.PackageManager

class SignatureChecker {
    fun check(pm: PackageManager) {
        val info = pm.getPackageInfo("com.example", PackageManager.GET_SIGNING_CERTIFICATES)
    }
}
