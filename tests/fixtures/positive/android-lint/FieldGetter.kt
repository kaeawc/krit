package com.example

class MyClass {
    private var count = 0

    fun getCount(): Int {
        return count
    }

    fun total(): Int {
        return getCount() + this.getCount()
    }
}
