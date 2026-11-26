package com.example.api

class Local {
    fun javaMarked(): Int = 0

    // Kotlin resolves this to the member above, not the marked top-level javaMarked.
    fun g() = javaMarked()

    class Nested {
        fun h() = javaMarked()
    }
}

class LocalMarked {
    @ExperimentalJava
    fun marked(): Int = 0

    fun g() = marked()
}

fun topLevelUse() = javaMarked()

class Outer {
    fun javaMarked(): Int = 0

    inner class Inner {
        // An inner class sees the outer member.
        fun h() = javaMarked()
    }
}
