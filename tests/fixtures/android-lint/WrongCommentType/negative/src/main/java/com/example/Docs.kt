package com.example

/** @property name a real KDoc */
class Docs {
    /* @return the value. */
    fun withPeriod(): Int = 1

    /* @Deprecated looks like an annotation */
    fun annotation() {}

    /* Just a comment mentioning user@example.com and [brackets] */
    fun plain() {}

    /* @return private stuff */
    private fun hidden(): Int = 1

    /* (non-Javadoc) @return x */
    fun nonJavadoc(): Int = 1

    // @return line comment
    fun lineComment(): Int = 1

    /* @return not attached */
    init {
        println()
    }

    fun body() {
        /* @return inside a body */
        val local = 1
    }
}
