package com.example

import android.os.Build
import android.os.Build.VERSION.SDK_INT

object Review {
    fun runEither(block: () -> Unit) {
        if (Build.VERSION.SDK_INT >= 31) {
            block()
        } else {
            block()
        }
    }

    fun runOther(block: () -> Unit, fallback: () -> Unit) {
        if (Build.VERSION.SDK_INT >= 31) {
            block()
        } else {
            fallback()
        }
    }

    fun shadowed(SDK_INT: Int) = SDK_INT >= 26

    fun imported() = SDK_INT >= 28
}
