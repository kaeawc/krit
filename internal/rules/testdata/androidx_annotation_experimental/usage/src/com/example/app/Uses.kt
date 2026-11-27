package com.example.app

import androidx.annotation.OptIn
import com.example.api.ExperimentalJava
import com.example.api.Holder
import com.example.api.MarkedClass
import com.example.api.WarnJava
import com.example.api.javaMarked
import com.example.api.kotlinMarked
import com.example.api.warnMarked

fun plain() {
    javaMarked()
    warnMarked()
    kotlinMarked()
    MarkedClass().member()
    Holder.inObject()
}

@OptIn(markerClass = [ExperimentalJava::class])
fun optedIn() {
    javaMarked()
    warnMarked()
}

@ExperimentalJava
fun propagated() {
    javaMarked()
}

@kotlin.OptIn(ExperimentalJava::class)
fun kotlinOptIn() {
    javaMarked()
}

@OptIn(markerClass = [WarnJava::class])
class OptedClass {
    fun use() = warnMarked()
}
