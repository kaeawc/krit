package com.example

import java.io.File

/* @constructor builds it */
class Edge /* @param trailing */ constructor() {
    /*
     * @param x the
     *   value. continues
     */
    fun dotOnContinuation(x: Int) {}

    /*
     * @Suppress uppercase first
     * @return lowercase after
     */
    fun upperThenLower(): Int = 1

    /*
     * @return first
     * Links to [Edge](https://example.com) later
     */
    fun linkAfterTag(): Int = 1

    /* see [Edge] and [File] */
    fun referenceOnly() {}

    /* @param a one */ /* second comment */
    fun twoComments(a: Int) {}

    /* @return one */
    @Deprecated("x")
    fun annotated(): Int = 1

    @Deprecated("x")
    /* @return after annotation */
    fun afterAnnotation(): Int = 1

    /* @param x secondary */
    constructor(x: Int) : this()

    /* @suppress companion */
    companion object {
        /* @return const */
        const val C = 1
    }

    /* @return protected */
    protected fun prot(): Int = 1

    /* @return internal */
    internal fun internalFun(): Int = 1

    /* @return getter */
    val computed: Int
        get() = 1
}

enum class Kind {
    /* @suppress entry */
    A,
    B,
}

interface Api {
    /* @return abstract */
    fun call(): Int
}

/* @return top level */
fun topLevel(): Int = 1

/* @return top level property */
val topProperty = 1

fun outer() {
    /* @suppress local class */
    class Local
}
