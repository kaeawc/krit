package com.example

import androidx.lifecycle.MutableLiveData

class SmartCasts {
    val live = MutableLiveData<String>()

    fun safeCallLambda(x: String?) {
        x?.let { live.value = x }
        x?.also { live.postValue(x) }
    }

    fun nullOrEmptyExit(x: String?) {
        if (x.isNullOrEmpty()) return
        live.value = x
    }

    fun nullOrBlankExit(x: String?) {
        if (x.isNullOrBlank()) {
            println("blank")
            return
        }
        live.value = x
    }

    fun orExit(x: String?, flag: Boolean) {
        if (flag || x == null) return
        live.value = x
    }

    fun requireCheck(x: String?, y: String?) {
        require(x != null)
        live.value = x
        check(y != null) { "y" }
        live.value = y
    }

    fun whenExit(x: String?) {
        when {
            x == null -> return
        }
        live.value = x
    }

    fun whenExitAfterOther(x: String?, flag: Boolean) {
        when {
            flag -> println("flag")
            x == null -> return
        }
        live.value = x
    }

    fun andIsNotAnExit(x: String?, flag: Boolean) {
        if (flag && x == null) return
        live.value = x
    }

    fun noExit(x: String?) {
        if (x == null) println("null")
        live.value = x
    }

    fun otherSafeCall(x: String?, y: String?) {
        y?.let { live.value = x }
    }
}
