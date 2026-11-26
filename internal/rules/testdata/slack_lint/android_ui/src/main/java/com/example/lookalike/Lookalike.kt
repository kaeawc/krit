package com.example.lookalike

import android.app.Activity

// A wrapper that only shares the Resources method's name.
interface ResourceProvider {
  fun getQuantityString(id: Int, count: Int, vararg args: Any): String
}

// A local type with Spanned-like constants.
object Spanned {
  const val SPAN_EXCLUSIVE_EXCLUSIVE = 33
}

class Holder(private val provider: ResourceProvider, private val owner: ContextOwner) {
  fun f(count: Int, flags: Int) {
    provider.getQuantityString(1, count, count)
    if (flags == Spanned.SPAN_EXCLUSIVE_EXCLUSIVE) {}
    val a = owner.getContext() as Activity
  }
}

interface ContextOwner {
  fun getContext(): Any
}
