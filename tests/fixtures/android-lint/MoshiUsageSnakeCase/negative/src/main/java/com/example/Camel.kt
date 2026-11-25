package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Camel(
  val userId: String,
  @Json(name = "snake_ok") val snake_ok: String,
)

// Not a Moshi class.
data class Plain(val user_id: String)
