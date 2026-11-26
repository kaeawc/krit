package com.example.lookalike

import com.example.lookalike.Guava.Preconditions

// Local functions that shadow the stdlib preconditions.
fun check(value: Boolean) {}
fun require(value: Boolean) {}

object Guava {
  object Preconditions {
    fun checkState(value: Boolean) {}
  }
}

class ConcurrentHashMap<K, V>

class Lookalike {
  fun use(x: Int?) {
    check(x != null)
    require(x != null)
    Preconditions.checkState(true)
    val m: ConcurrentHashMap<String?, Int?> = ConcurrentHashMap()
  }
}
