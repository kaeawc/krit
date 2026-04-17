package com.example

class MyClass {
    fun process(items: List<Item>) {
        // getter calls outside of loops are fine
        val name = item.getName()
        val value = item.getValue()
        println("$name=$value")
    }

    fun noLoop() {
        val x = list.getCount()
    }
}
