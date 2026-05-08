package com.example

class MyClass {
    fun process(items: List<Item>) {
        for (item in items) {
            val name = item.getName()
            val value = item.getValue()
            println("$name=$value")
        }
    }

    fun loop() {
        var i = 0
        while (i < list.size) {
            val x = list.getCount()
            i++
        }
    }

    fun getterInNestedLoop(matrix: Array<IntArray>) {
        for (row in matrix) {
            for (value in row) {
                val childCount = view.getChildCount()
            }
        }
    }
}
