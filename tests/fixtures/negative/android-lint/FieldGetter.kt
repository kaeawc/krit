package com.example

class MyClass(private val other: MyClass) {
    private var count = 0

    fun getCount(): Int {
        return count + 1
    }

    fun total(item: Item): Int {
        // Getter has logic, and the other calls are on other receivers.
        item.getName()
        other.getCount()
        return getCount()
    }
}
