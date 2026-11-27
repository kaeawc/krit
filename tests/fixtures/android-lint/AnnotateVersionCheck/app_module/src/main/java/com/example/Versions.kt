package com.example

import android.os.Build

object Versions {
    fun isAtLeastO(): Boolean {
        return Build.VERSION.SDK_INT >= Build.VERSION_CODES.O
    }

    fun isAtLeastP() = Build.VERSION.SDK_INT >= 28

    fun isAfterQ(): Boolean {
        return Build.VERSION.SDK_INT > Build.VERSION_CODES.Q
    }

    fun isAtLeast(api: Int): Boolean {
        return Build.VERSION.SDK_INT >= api
    }

    fun runOnS(block: Runnable) {
        if (Build.VERSION.SDK_INT >= 31) {
            block.run()
        }
    }

    @JvmField
    val IS_AT_LEAST_T = Build.VERSION.SDK_INT >= 33
}
