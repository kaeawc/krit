// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 15, 21, 45, 48, 51, 55, 63, 67, 75, 78, 82, 87, 92, 97, 100, 106, 122, 124, 126, 130, 138, 145
// Main's K2/K4 Lint parity bodies, including last-return-wins compatibility.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

val v1 = object : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean {
    return true
  }
}
val v2 = object : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return false
    return true
  }
}
val v3 = object : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean {
    println(h)
    return true
  }
}
val v4 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return true
    return false
  }
}
val v5 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return false
  }
}
val v6 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    throw IllegalStateException()
  }
}
val v7 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession) = (h.length > 0)
}
val v8 = <!BadHostnameVerifier!>HostnameVerifier { _, _ -> true }<!>

class V9 : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
}
class V10 : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
  fun other() { println("x") }
}
open class V11 : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String?, s: SSLSession?): Boolean = true
}
class V12 : V11()
class V13 : V11() {
  <!BadHostnameVerifier!>override<!> fun verify(h: String?, s: SSLSession?): Boolean { return true }
}
abstract class V14 : HostnameVerifier
interface V15 : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
}
class V16 : HostnameVerifier {
  fun verify(h: String): Boolean = true
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
}
class V17 : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean { return (true) }
}

open class Base
class P1 : Base(), HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
}
class P2(val a: Int) : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
}
class P3 : HostnameVerifier {
  constructor()
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
}
class P4 : HostnameVerifier {
  init { }
  val x = 1
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
}
class P5 : HostnameVerifier {
  val x = compute()
  fun compute() = 1
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
}
class P6 : HostnameVerifier by Impl() {
}
class Impl : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
}
class P7 : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean {
    val l = listOf(1)
    return true
  }
}
class P8 : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean {
    if (h.isEmpty()) throw RuntimeException()
    return true
  }
}
class P9 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = when { true -> true else -> true }
}
class P10 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return true
    return false
  }
}
class P11 : HostnameVerifier {
  companion object { fun f() = println() }
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
}
val lam = <!BadHostnameVerifier!>HostnameVerifier { h, s -> true }<!>
fun fn(): HostnameVerifier = object : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession) = true
}

class MultipleTrue : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean {
    if (h.isEmpty()) return true
    println(h)
    return true
  }
}

class EarlierNonliteral : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean {
    if (h.isEmpty()) return h == s.peerHost
    return true
  }
}

class InlineNonlocalReturn : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean {
    run { return true }
  }
}
