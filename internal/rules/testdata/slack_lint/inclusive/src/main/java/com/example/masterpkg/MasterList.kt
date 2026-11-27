package com.example.masterpkg

/** KDoc mentioning master */
@Suppress("unused")
class MasterView(val slaveCount: Int, masterName: String) {
  private val whitelistEntries = listOf("a")
  var masterSlave = 0

  // master in a comment
  fun syncMaster(slaveId: Int): String {
    val masterCopy = slaveId
    listOf(1).forEach masterLoop@{ whitelistItem ->
      if (whitelistItem > masterCopy) return@masterLoop
    }
    val (masterA, b) = Pair(1, 2)
    try { } catch (masterError: Exception) { }
    for (slaveIndex in 0..1) { }
    return "master"
  }

  constructor(master: Boolean) : this(0, "")

  object MasterHolder
  companion object Slaves
  enum class Mode { MASTER, Replica }
  interface BlacklistPolicy
  fun blacklisted() = Unit
  fun BLACKLIST() = Unit
}

typealias MasterAlias = String
fun topMaster() = object : Runnable { override fun run() {} }
val topSlave = 1
