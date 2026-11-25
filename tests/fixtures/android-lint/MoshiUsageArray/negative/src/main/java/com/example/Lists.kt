package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Lists(val names: List<String>, val ids: List<Int>) {
  // Not a constructor property.
  val cached: Array<String> = emptyArray()
}

// Not a Moshi class.
data class Plain(val names: Array<String>)
