package com.example

import android.content.Context
import android.graphics.Canvas
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView
import com.example.databinding.ItemBinding

fun ViewGroup.inflate(id: Int): View = this

open class Deco(private val inflater: LayoutInflater) : RecyclerView.ItemDecoration() {
  override fun onDraw(c: Canvas, parent: RecyclerView) {
    LayoutInflater.from(parent.context).inflate(1, parent, false)
    ItemBinding.inflate(LayoutInflater.from(parent.context))
    ItemBinding.inflate(inflater, parent, false)
    ItemBinding.bind(parent)
    inflater.inflate(1, null)
    parent.inflate(1)
    View.inflate(parent.context, 1, null)
    run { inflater.inflate(1, null) }
    val o = object : Any() {
      fun g() { inflater.inflate(1, null) }
    }
  }

  class Nested {
    fun h(inflater: LayoutInflater) { inflater.inflate(1, null) }
  }
}

class Deco2(inflater: LayoutInflater) : Deco(inflater) {
  fun k(inflater: LayoutInflater) { inflater.inflate(1, null) }
}

class NotDeco(private val inflater: LayoutInflater) {
  fun f(parent: ViewGroup) {
    inflater.inflate(1, parent, false)
    ItemBinding.inflate(inflater)
  }
}

open class MyView(context: Context) : View(context) {
  override fun toString(): String = "MyView"
  fun self() {
    toString()
    this.toString()
  }
}

class SubView(context: Context) : MyView(context)

class Plain {
  override fun toString(): String = "Plain"
}

fun strings(v: View, m: MyView, s: SubView, p: Plain, t: android.widget.TextView) {
  v.toString()
  m.toString()
  s.toString()
  p.toString()
  t.toString()
  "$m"
  m.hashCode()
}
