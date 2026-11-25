package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Duplicates(
  @Json(name = "dup") val first: Int,
  @Json(name = "dup") val second: Int,
  val plain: Int,
  @Json(name = "plain") val renamedToPlain: Int,
)

@JsonClass(generateAdapter = false)
enum class DuplicateEnum { UNKNOWN, @Json(name = "x") ONE, @Json(name = "x") TWO }
