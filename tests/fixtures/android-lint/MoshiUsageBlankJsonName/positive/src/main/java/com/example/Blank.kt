package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Blank(@Json(name = "") val a: Int, @Json(name = " ") val b: Int)

@JsonClass(generateAdapter = false)
enum class BlankEnum { UNKNOWN, @Json(name = "") ONE }
