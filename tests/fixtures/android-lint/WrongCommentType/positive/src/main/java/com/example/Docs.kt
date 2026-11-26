package com.example

/* @property name the name */
class Docs(
    /* @param value the value */
    val value: Int,
) {
    /*
     * Computes it.
     * @return the result
     */
    fun compute(): Int = value

    /* @see Docs */
    val field = 1

    /* Links to [Docs](https://example.com) here */
    fun linked() {}

    /*
     * @param x first
     * @throws IllegalStateException when bad
     */
    fun multi(x: Int) {}
}

/* @suppress */
object Single
