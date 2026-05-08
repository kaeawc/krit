package com.example.redundant

public class Foo constructor(val name: String) {

    fun bar(): Unit {
        val greeting = "hello".orEmpty()
        val items = listOf(1, 2, 3)
        val x = items.apply { }
        val transform = { it -> it + 1 }
        val trailing = "whitespace"   
	    val tabbed = true
    }
}
