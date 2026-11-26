package com.example

class Bind {
    /* @return blank line after */

    fun blankAfter(): Int = 1

    /* @return first of two lines */
    /* second plain */
    fun twoLines(): Int = 1

    // line comment first
    /* @return after line comment */
    fun lineThenBlock(): Int = 1

    /* @return before line comment */
    // trailing line comment
    fun blockThenLine(): Int = 1

    /* plain first */
    /* @return second has tag */
    fun tagInSecond(): Int = 1

    var withField: Int = 0
        /* @return accessor */
        get() = field

    /* @return backing field via accessor */
    var accessorField: Int = 0
        get() = field + 1

    /* @return lateinit */
    lateinit var late: String

    /* @return delegated */
    val lazyValue by lazy { 1 }

    /* @return abstract-like no field */
    val noField: Int get() = 2
}

abstract class Abs {
    /* @return abstract property */
    abstract val abs: Int
}
