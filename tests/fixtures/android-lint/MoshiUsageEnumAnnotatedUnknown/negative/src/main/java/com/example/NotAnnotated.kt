package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = false)
enum class NotAnnotated {
  UNKNOWN,
  @Json(name = "one") ONE,
}

annotation class Documented

@JsonClass(generateAdapter = false)
enum class OtherAnnotation {
  @Documented UNKNOWN,
  ONE,
}
