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

    fun mapGetOrDefault() {
        for (key in keys) {
            // map.getOrDefault is NOT a field getter
            val value = map.getOrDefault(key, 0)
        }
    }

    fun listGetOrNull() {
        for (i in 0..10) {
            // list.getOrNull is NOT a field getter
            val item = list.getOrNull(i)
        }
    }

    fun getOrElseInLoop() {
        for (item in items) {
            // getOrElse is NOT a field getter
            val value = item.getOrElse { "default" }
        }
    }

    fun getOrPutInLoop() {
        for (key in keys) {
            // getOrPut is NOT a field getter
            val value = cache.getOrPut(key) { compute(key) }
        }
    }

    fun getKeyAndValueInLoop() {
        for (entry in map.entries) {
            // getKey and getValue are special Kotlin methods, not field getters
            val k = entry.getKey()
            val v = entry.getValue()
        }
    }
}
