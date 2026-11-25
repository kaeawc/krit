package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Redundant(@Json(name = "id") val id: String)

@JsonClass(generateAdapter = false)
enum class RedundantEnum { UNKNOWN, @Json(name = "ONE") ONE }
