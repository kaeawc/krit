package com.example

import slack.lint.annotations.MustUseNamedParams
import slack.lint.annotations.RestrictCallsTo

@MustUseNamedParams
fun configure(name: String, size: Int) {}

class Builder @MustUseNamedParams constructor(val a: Int, val b: Int)

interface Callback {
  @RestrictCallsTo
  fun onEvent(value: Int)
}

class Impl : Callback {
  override fun onEvent(value: Int) {}
}

fun sameFile(cb: Callback) {
  cb.onEvent(1)
  configure("x", 1)
}
