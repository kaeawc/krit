package com.example.naming

fun example() {
    val x = 1
    val items = listOf(1, 2, 3)
    items.forEach { x ->
        println(x)
    }

    val name = "outer"
    items.map { name ->
        name.toString()
    }
}
