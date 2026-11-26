package com.example

import android.app.Activity
import android.text.Spanned
import android.view.View

fun f(v: View, flags: Int): Boolean {
  val a = v.getContext() is Activity
  val b = (flags or Spanned.SPAN_POINT_MARK_MASK) == Spanned.SPAN_EXCLUSIVE_EXCLUSIVE
  val c = (flags and Spanned.SPAN_POINT_MARK_MASK) == Spanned.SPAN_EXCLUSIVE_EXCLUSIVE
  return a && b && c
}
