package com.example

import android.os.Build

fun andRight(x: Boolean) = x && Build.VERSION.SDK_INT >= 26

fun andLeft(x: Boolean) = Build.VERSION.SDK_INT >= 26 && x

fun outer(): Boolean {
    fun isO() = Build.VERSION.SDK_INT >= 26
    return isO()
}

inline fun runIfS(block: () -> Unit) {
    if (Build.VERSION.SDK_INT >= 31) block()
}

inline fun runIfSRun(block: Runnable) {
    if (Build.VERSION.SDK_INT >= 31) block.run()
}

fun parens(): Boolean = (Build.VERSION.SDK_INT >= 28)
