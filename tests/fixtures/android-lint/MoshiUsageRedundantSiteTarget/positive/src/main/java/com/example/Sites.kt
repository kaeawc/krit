package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Sites(
  @field:Json(name = "a") val a: Int,
  @Json(name = "b") @field:Json(name = "b") val b: Int,
  @field:Json(name = "c") @get:Json(name = "c") val c: Int,
)
