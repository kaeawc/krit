package com.example

import androidx.lifecycle.LiveData
import androidx.lifecycle.MutableLiveData

class Holder {
    val names = MutableLiveData<String>()
    val nullable = MutableLiveData<String?>()
    val typed: MutableLiveData<Int> = MutableLiveData()
    private val backing = MutableLiveData<Boolean>()
    val exposed: LiveData<Boolean> = backing

    fun update(maybe: String?, sure: String, n: Int?) {
        names.value = null
        names.value = maybe
        names.value = sure
        names.postValue(null)
        names.postValue(maybe)
        names.setValue(maybe)
        nullable.value = null
        nullable.value = maybe
        typed.value = n
        typed.value = 1
        backing.value = null
        if (maybe != null) names.value = maybe
        names.value = maybe ?: "x"
        names.value = maybe!!
        names.value = lookup()
        names.value = lookupSure()
    }

    fun lookup(): String? = null
    fun lookupSure(): String = ""
}

fun local(maybe: String?) {
    val live = MutableLiveData<String>()
    live.value = maybe
    val lazy = MutableLiveData<String>("init")
    lazy.value = null
}

class Sub : MutableLiveData<String>()

fun sub(s: Sub, maybe: String?) {
    s.value = maybe
}

fun <T> generic(live: MutableLiveData<T>, v: T?) {
    live.value = v
}
