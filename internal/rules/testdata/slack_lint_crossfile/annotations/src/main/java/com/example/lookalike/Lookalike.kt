package com.example.lookalike

// Same names as the annotated declarations, not annotated.
fun configure(name: String, size: Int) {}

interface Callback { fun onEvent(value: Int) }

class Unrelated {
  fun onEvent(value: Int) {}
}

fun use(cb: Callback, other: Unrelated) {
  configure("x", 1)
  cb.onEvent(1)
  other.onEvent(2)
}
