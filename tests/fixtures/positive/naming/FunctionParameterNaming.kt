package com.example.naming

fun foo(BadParam: Int, AnotherBad: String) {
    println("$BadParam $AnotherBad")
}

fun bar(WrongName: Double) {
    println(WrongName)
}
