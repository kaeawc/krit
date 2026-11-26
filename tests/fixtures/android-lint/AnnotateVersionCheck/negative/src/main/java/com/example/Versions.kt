package com.example

import android.os.Build
import androidx.annotation.ChecksSdkIntAtLeast

object Versions {
    @ChecksSdkIntAtLeast(api = Build.VERSION_CODES.O)
    fun isAtLeastO(): Boolean {
        return Build.VERSION.SDK_INT >= Build.VERSION_CODES.O
    }

    private fun isPrivate() = Build.VERSION.SDK_INT >= 28

    fun isBelow(): Boolean {
        return Build.VERSION.SDK_INT < 28
    }

    fun twoStatements(): Boolean {
        val x = 1
        return Build.VERSION.SDK_INT >= 28 + x
    }

    fun current() = Build.VERSION.SDK_INT
}
