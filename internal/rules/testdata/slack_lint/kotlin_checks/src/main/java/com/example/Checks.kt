package com.example

import android.os.Parcelable
import com.google.common.base.Preconditions
import com.google.common.base.Preconditions.checkArgument
import java.util.concurrent.ConcurrentHashMap
import kotlinx.parcelize.IgnoredOnParcel
import kotlinx.parcelize.Parcelize

class K(private val map: ConcurrentHashMap<String?, Int?>) {
  val a: ConcurrentHashMap<String, Int?> = ConcurrentHashMap()
  fun f(s: String?, list: List<String>?) {
    s!!.length
    val x = s!!
    list!![0]
    check(s != null)
    check(s != null) { "msg" }
    checkNotNull(s)
    require(s.isNotEmpty())
    requireNotNull(s) { "x" }
    check(true, { "positional" })
    kotlin.check(false)
    Preconditions.checkState(true)
    checkArgument(false)
    Preconditions.checkElementIndex(1, 2)
    val m = ConcurrentHashMap<String?, Int>()
    val m2: ConcurrentHashMap<String, Int> = ConcurrentHashMap<String, Int?>()
  }
}

@Parcelize
class P(
  val callback: () -> Unit,
  val nullable: (() -> Unit)?,
  val name: String,
  val f1: Function1<Int, String>,
  @IgnoredOnParcel val ignored: () -> Unit,
  plain: (Int) -> Unit,
  val suspending: suspend () -> Unit,
) : Parcelable

typealias Callback = (String) -> Unit

@Parcelize
class Q(val alias: Callback, val qualified: kotlin.Function0<Unit>, val list: List<() -> Unit>) : Parcelable

class NotParcelized(val callback: () -> Unit)

class More {
  fun bangs(a: A?) {
    a!!.b = 1
    a!!!!
  }

  fun preconditions(x: Int?) {
    requireNotNull<Int>(x) { "typed" }
    requireNotNull<Int>(x)
    check(value = true, lazyMessage = { "named" })
    java.util.Objects.requireNonNull(x)
  }

  fun maps(param: ConcurrentHashMap<String?, String>?) {
    val other = java.util.concurrent.ConcurrentHashMap<String?, String?>()
    val ok = ConcurrentHashMap<String, String>()
  }
}

class A { var b = 0 }
