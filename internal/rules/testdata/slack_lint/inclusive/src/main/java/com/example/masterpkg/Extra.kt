package com.example.other

abstract class Base {
  abstract val masterAbstract: Int
  val masterComputed: Int get() = 1
  val masterLazy by lazy { 1 }
  internal val masterInternal = 1
  internal fun masterInternalFun() {}
  protected val masterProtected = 1
  var masterBacked: Int = 0
    get() = field
    set(slaveValue) { field = slaveValue }
  fun run() {
    val masterDelegated by lazy { 2 }
  }
}

data class Pairing(val masterOne: Int, private val masterTwo: Int)

@Suppress("unused")
class Annotated
