package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Vals(val a: Int) {
  // Body properties are not constructor properties.
  var cached: Int = 0
}

// Not a Moshi class.
data class Plain(var a: Int)

// generateAdapter = false returns before the constructor checks.
@JsonClass(generateAdapter = false)
class KeptForR8(var a: Int)
