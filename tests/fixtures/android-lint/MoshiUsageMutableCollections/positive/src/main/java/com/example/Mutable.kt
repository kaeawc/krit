package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Mutable(
  val list: MutableList<String>,
  val set: MutableSet<String>,
  val map: MutableMap<String, Int>,
  val collection: MutableCollection<String>,
)
