package com.example

import androidx.lifecycle.MutableLiveData as MLD

class Aliased {
    val aliased = MLD<String>()
    val typed: MLD<String> = MLD()

    fun update() {
        aliased.value = null
        typed.value = null
    }
}
