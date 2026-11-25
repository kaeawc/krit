package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Renamed(@Json(name = "user_id") val userId: String)

@JsonClass(generateAdapter = false)
enum class RenamedEnum { UNKNOWN, @Json(name = "one") ONE }
