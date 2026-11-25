package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Unique(@Json(name = "a") val first: Int, @Json(name = "b") val second: Int)

// UNKNOWN is skipped, so another member may use its name.
@JsonClass(generateAdapter = false)
enum class UnknownName { UNKNOWN, @Json(name = "UNKNOWN") OTHER }

// Ignored members have no JSON name of their own.
@JsonClass(generateAdapter = true)
data class Ignored(val a: Int, @Json(ignore = true) val b: Int = 0, @Json(ignore = true) val c: Int = 0)
