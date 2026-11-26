package com.example

import android.app.Activity
import android.app.Fragment
import android.content.Context
import android.content.res.Resources
import android.text.Spannable
import android.text.Spanned
import android.text.Spanned.SPAN_EXCLUSIVE_EXCLUSIVE
import android.view.View
import android.widget.TextView

class Ui(private val view: View, private val text: TextView, private val res: Resources, private val fragment: Fragment) {
  fun casts() {
    val a = view.getContext() as Activity
    val b = view.context as Activity
    val c = text.getContext() as Activity
    val d = view.getContext() as? Activity
    val e = (view.getContext()) as Activity
    val f = view.getContext() as Context
    val g = fragment.getActivity() as Activity
  }

  fun quantity(count: Int) {
    res.getQuantityString(R.plurals.items, count, count)
    res.getQuantityString(R.plurals.items, count, getFormattedCount(count))
    val formatted = getFormattedCount(count)
    res.getQuantityString(R.plurals.items, count, formatted, "x")
    res.getQuantityString(R.plurals.items, count)
    view.resources.getQuantityString(R.plurals.items, count, count)
  }

  fun spans(flags: Int, s: Spanned) {
    if (flags == Spanned.SPAN_EXCLUSIVE_EXCLUSIVE) {}
    if (Spannable.SPAN_INCLUSIVE_INCLUSIVE != flags) {}
    if (flags and Spanned.SPAN_POINT_MARK_MASK == Spanned.SPAN_EXCLUSIVE_INCLUSIVE) {}
    if ((flags and Spanned.SPAN_POINT_MARK_MASK) == Spanned.SPAN_EXCLUSIVE_EXCLUSIVE) {}
    if (s.getSpanFlags(this) == SPAN_EXCLUSIVE_EXCLUSIVE) {}
    if (flags === Spanned.SPAN_INCLUSIVE_EXCLUSIVE) {}
    if (flags == Spanned.SPAN_COMPOSING) {}
    if (Spanned.SPAN_EXCLUSIVE_EXCLUSIVE == Spanned.SPAN_INCLUSIVE_INCLUSIVE) {}
  }
}

class CustomView(context: Context) : View(context) {
  fun host() = getContext() as Activity
}

fun getFormattedCount(count: Int): String = count.toString()

object R { object plurals { const val items = 1 } }
