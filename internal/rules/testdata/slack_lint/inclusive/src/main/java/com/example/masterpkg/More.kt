package com.example.masterpkg

import kotlin.jvm.JvmField

/** Doc */
interface Replica {
  val slaveProp: Int
  fun slaveFun()
}

@Suppress("unused")
object Holder {
  /** doc */
  private val masterPrivate = 1
  const val MASTER_CONST = 1
  @JvmField val masterField = 2
  lateinit var masterLate: String
  val Int.masterExt: Int get() = this
  var slaveSetter: Int = 0
    private set

  /** Function doc */
  @JvmStatic
  fun masterDoc() {
    fun slaveLocal() {}
    val f = { (masterKey, v): Pair<Int, Int> -> masterKey }
    when (val masterWhen = 1) { else -> masterWhen }
    outer@ for (i in 0..1) { break@outer }
    masterLoop@ while (true) { break@masterLoop }
  }

  class Inner<MasterT>
}
