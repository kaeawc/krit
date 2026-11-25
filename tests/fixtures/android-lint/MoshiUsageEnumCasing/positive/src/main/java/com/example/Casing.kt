package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = false)
enum class Casing {
  UNKNOWN,
  lower,
  camelCase,
  @Json(name = "annotated") alreadyAnnotated,
}
