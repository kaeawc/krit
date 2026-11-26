package com.example

import androidx.lifecycle.MutableLiveData

class MoreSmartCasts {
    val live = MutableLiveData<String>()

    fun log() {}

    fun notNullOrEmpty(x: String?) {
        if (!x.isNullOrEmpty()) live.value = x
    }

    fun notNullOrBlank(x: String?) {
        if (!x.isNullOrBlank()) live.value = x
    }

    fun elseBranch(x: String?) {
        if (x == null) {
            log()
        } else {
            live.value = x
        }
    }

    fun errorExit(x: String?) {
        if (x == null) error("missing")
        live.value = x
    }

    fun todoExit(x: String?) {
        if (x == null) TODO()
        live.value = x
    }

    fun elvisError(x: String?) {
        val y = x ?: error("missing")
        live.value = x
        println(y)
    }

    fun whenSubject(x: String?) {
        when (x) {
            null -> return
            else -> live.value = x
        }
    }

    fun elvisRunReturn(x: String?) {
        x ?: run { return }
        live.value = x
    }

    fun thenBranchOfNullTest(x: String?) {
        if (x == null) {
            live.value = x
        } else {
            log()
        }
    }

    fun nonExitingCall(x: String?) {
        if (x == null) log()
        live.value = x
    }
}
