package com.example.api

@ExperimentalJava
fun javaMarked(): Int = 1

@WarnJava
fun warnMarked(): Int = 2

@ExperimentalKotlin
fun kotlinMarked(): Int = 3

@ExperimentalJava
class MarkedClass {
    fun member(): Int = 4
}

object Holder {
    @ExperimentalJava
    fun inObject(): Int = 5
}
