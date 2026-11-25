package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Snake(
  val user_id: String,
  val kebab_and_more_words: String,
  val _leading: String,
  @Json(name = "x") @field:Json(name = "x") val multi_annotated: String,
)
