package com.example.naming

fun example() {
    val name = "outer"
    run {
        val name = "inner"
        println(name)
    }
}
