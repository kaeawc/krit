package com.example.sample

import java.util.*
import kotlin.collections.*

// TODO: fix this later
class sample_class {

    protected val myVar = "hello"
    var unusedVar = 42

    fun longFunctionName() {
        val x = 3
        val y = 4
        val MAGIC = 42
        if (x > 0) {
            if (y > 0) {
                if (x > y) {
                    if (y > 1) {
                        if (x > 2) {
                            println("deep")
                        }
                    }
                }
            }
        }
    }

    fun equals(other: String): Boolean {
        return true
    }

    fun Unused_Function() {
        try {
            val result = mapOf("a" to 1)["a"]!!
            result.printStackTrace()
        } catch (e: Exception) {
            // swallowed
        } finally {
            return
        }
    }

    fun tooManyReturns(x: Int): String {
        if (x > 0) return "positive"
        if (x < 0) return "negative"
        return "zero"
    }

    fun withVoid(): Void? = null

    companion object {
        val CONSTANT = "hello"
    }
}

fun topLevel_bad() {
    val list = listOf(1, 2, 3)
    list.filter { it > 1 }.first()
    val arr = Array<Int>(5) { it }
    val check = list.find { it > 2 } != null
    check(check != null)
}
