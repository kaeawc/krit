package com.example

import java.io.Serializable
import java.util.Date
import kotlin.collections.ArrayList

public class Foo constructor(val name: String) {

    fun bar(): Unit {
        val x = "hello"
        val list = listOf()
        val set = setOf()
        val map = mapOf()
        val items = listOf(1, 2, 3)
        val result = items.find { it > 2 } != null
        val result2 = items.find { it < 0 } == null
        val filtered = items.filter { it > 1 }.first()
        val filtered2 = items.filter { it > 2 }.count()
        val sorted = items.sorted().reversed()
        val sortedBy = items.sortedBy(it).reversed()
        val size = items.flatMap { it.children }.size
        val arr: Array<Int> = intArrayOf(1, 2, 3)
        val arr2: Array<Boolean> = booleanArrayOf()
        val greeting = "hello".orEmpty()
        val x2 = items.apply { }
        val thing = { it -> it + 1 }
        for (i in 0 until 10) {
            println(i)
        }
        val s: String? = null
        if (s == null || s.isEmpty()) {
            println("empty")
        }
        val maybeList: List<String>? = null
        val safe = maybeList ?: emptyList()
        val value = items.get(0)
        if (!isValid) throw IllegalArgumentException("invalid input")
        if (!isReady) throw IllegalStateException("not ready")
	    val tabbed = true
        val trailing = "whitespace"
    }
}
