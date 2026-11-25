package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Immutable(
  val list: List<String>,
  val map: Map<String, Int>,
  // Lint only checks the property's type as written, so a nested mutable type is not reported.
  val nested: List<MutableList<String>>,
)
