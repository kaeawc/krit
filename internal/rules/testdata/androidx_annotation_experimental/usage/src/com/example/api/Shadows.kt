package com.example.api

class Shadows {
    fun fromBody(): Int = javaMarked()

    class Nested {
        fun fromNested(): Int = javaMarked()
    }

    companion object {
        fun javaMarked(): Int = 0
    }
}

class PropertyShadow {
    val javaMarked = { 0 }
    fun use(): Int = javaMarked()
}

fun localFun(): Int {
    fun javaMarked(): Int = 0
    return javaMarked()
}

fun parameter(javaMarked: () -> Int): Int = javaMarked()

fun localVal(): Int {
    val javaMarked = { 0 }
    return javaMarked()
}

val lambdaParam: (() -> Int) -> Int = { javaMarked -> javaMarked() }

fun afterNestedLocal(): Int {
    run {
        fun javaMarked(): Int = 0
        javaMarked()
    }
    return javaMarked()
}

fun beforeLocalVal(): Int {
    val first = javaMarked()
    val javaMarked = { 0 }
    return first + javaMarked()
}
