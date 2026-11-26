package com.example

import com.other.MutableLiveData

class Box : MutableLiveData<String>()

class Outer : androidx.lifecycle.MutableLiveData<String?>() {
    class Inner : androidx.lifecycle.MutableLiveData<String>()
}

fun lookalike(b: Box, o: Outer) {
    b.value = null
    o.value = null
}
