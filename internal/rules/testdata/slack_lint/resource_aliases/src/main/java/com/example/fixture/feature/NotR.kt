package com.example.fixture.feature

import android.os.Build

object Outer {
  object R {
    const val x = 1
  }
}

fun notResourceClasses(): String {
  val a = Outer.R.x
  return Build.VERSION_CODES.R.toString() + a
}
