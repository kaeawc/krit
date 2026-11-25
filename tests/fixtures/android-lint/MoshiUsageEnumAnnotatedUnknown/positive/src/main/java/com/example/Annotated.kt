package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = false)
enum class Annotated {
  @Json(name = "unknown") UNKNOWN,
  ONE,
}
