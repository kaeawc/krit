package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = false)
enum class NoUnknown { ONE, TWO }

@JsonClass(generateAdapter = false)
enum class LateUnknown { ONE, UNKNOWN }

class Outer {
  @JsonClass(generateAdapter = false)
  enum class Nested(val code: Int) {
    ONE(1),
    UNKNOWN(0);

    fun isKnown() = this != UNKNOWN
  }
}
