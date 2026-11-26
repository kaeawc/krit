package com.example.other

import com.example.Builder
import com.example.Callback
import com.example.Impl
import com.example.configure

fun use(cb: Callback, impl: Impl) {
  configure("x", 1)
  configure(name = "x", size = 1)
  configure("x", size = 1)
  configure()
  Builder(1, 2)
  Builder(a = 1, b = 2)
  cb.onEvent(1)
  impl.onEvent(2)
}

class Sub : Callback {
  override fun onEvent(value: Int) {}
  fun self() {
    onEvent(3)
  }
}

fun trailing(block: () -> Unit) {}
