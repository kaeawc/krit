package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Model(val a: Int)

class Outer {
  // Nested, not inner.
  @JsonClass(generateAdapter = true)
  data class Nested(val a: Int)
}

// moshi-sealed owns sealed types with a custom generator.
@JsonClass(generateAdapter = true, generator = "sealed:type")
sealed class Sealed
