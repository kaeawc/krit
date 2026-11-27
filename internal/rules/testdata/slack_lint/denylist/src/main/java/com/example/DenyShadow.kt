package com.example

import com.other.Date
import com.other.Schedulers

fun <T> runCatching(block: () -> T): T = block()

class DenyShadow {
  fun lookalikes(view: com.other.View) {
    val d = Date()
    d.getTime()
    Schedulers.newThread()
    runCatching { 1 }
    view.setOnClickListener(null)
  }

  fun launchIn(x: Int) = x

  fun memberShadow() {
    launchIn(1)
  }
}
