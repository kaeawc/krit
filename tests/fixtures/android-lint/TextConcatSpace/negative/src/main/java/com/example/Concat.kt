package com.example

val a = "This is the first " +
    "line of text"
val b = "This is the first" + "line on the same line"
val c = "single" +
    "word"
val d = "ends with digit 2" +
    "next"
val e = "ends with period." +
    "Next"
val f = "words here" +
    " starts with space"
val g = "words here" +
    "1 starts with digit"
val n = 3
val h = "words here" +
    n
val i = "words here" -
    "x"

operator fun String.minus(other: String) = this
