package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Arrays(
  val names: Array<String>,
  val ids: IntArray,
  val nested: List<Array<String>>,
)
