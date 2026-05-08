package com.example.collections

class CollectionIdioms {

    fun findToAnyNone(items: List<Int>) {
        val hasPositive = items.find { it > 0 } != null
        val noNegative = items.find { it < 0 } == null
    }

    fun filterChains(items: List<Int>) {
        val first = items.filter { it > 5 }.first()
        val last = items.filter { it < 10 }.last()
        val count = items.filter { it % 2 == 0 }.count()
    }

    fun emptyCollections() {
        val strings = listOf()
        val numbers = setOf()
        val mapping = mapOf()
    }

    fun sortReversed(items: List<Int>) {
        val desc = items.sorted().reversed()
    }

    fun elementAccess(items: List<String>, map: Map<String, Int>) {
        val elem = items.get(0)
        val value = map.get("key")
    }

    fun redundantMap(items: List<String>) {
        val same = items.map { it }
    }

    fun primitiveArrays() {
        val ints: Array<Int> = arrayOf(1, 2, 3)
        val bools: Array<Boolean> = arrayOf(true, false)
    }
}
