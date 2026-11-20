// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 18, 24, 48, 51, 54, 58, 66, 70, 78, 81, 85, 90, 95, 100, 103, 110, 126, 128, 130, 134, 144, 151, 157, 183
// FIR requires every return path to be the literal true; Go retains Lint's
// class-wide last-return-wins compatibility on the pinned divergences below.
package test

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

val v1 = object : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean {
    return true
  }
}
// FIR is silent because the false return rejects a host; Go's Lint visitor
// reports the final true return.
val v2 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
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
// Go reports the final true but FIR rejects the explicit throwing path.
class P8 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h.isEmpty()) throw RuntimeException()
    return true
  }
}
class P9 : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = when { true -> true else -> true }
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

// Go reports the final true but FIR also requires the earlier return value
// to be the literal true.
class EarlierNonliteral : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h.isEmpty()) return h == s.peerHost
    return true
  }
}

class InlineNonlocalReturn : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean {
    run { return true }
  }
}

class ElvisInlineNonlocalReturn : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean {
    h.takeIf { it.isNotEmpty() } ?: run { return true }
    return true
  }
}

// Go reports the class-member last-return case; FIR sees the false path and
// rejects it. Enum, entry, and constructor variants below pin strict FIR for
// owner shapes where Go's legacy fallback can instead remain silent.
enum class EnumFalseThenTrue : HostnameVerifier {
  INSTANCE;

  override fun verify(h: String, s: SSLSession): Boolean {
    if (h != "x") return false
    return true
  }
}

enum class EntryFalseThenTrue : HostnameVerifier {
  INSTANCE {
    override fun verify(h: String, s: SSLSession): Boolean {
      if (h != "x") return false
      return true
    }
  };

  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean = true
}

open class OtherBase

class ConstructorFalseThenTrue : OtherBase(), HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h != "x") return false
    return true
  }
}

class SecondaryFalseThenTrue(val marker: Int) : HostnameVerifier {
  constructor() : this(0)

  override fun verify(h: String, s: SSLSession): Boolean {
    if (h != "x") return false
    return true
  }
}

abstract class VerifierBase : HostnameVerifier

class DerivedFalseThenTrue : VerifierBase() {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h != "x") return false
    return true
  }
}

class BranchResults : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean =
    if (h.isEmpty()) true else h == s.peerHost
}
class AllTrueBranches : HostnameVerifier {
  <!BadHostnameVerifier!>override<!> fun verify(h: String, s: SSLSession): Boolean =
    if (h.isEmpty()) true else true
}
