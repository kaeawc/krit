package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Props(
  val a: Int,
  internal val b: Int,
  public val c: Int,
)

// Not a Moshi class.
data class Plain(private val a: Int)
