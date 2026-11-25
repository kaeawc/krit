package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Named(@Json(name = "x") val a: Int, @Json(ignore = true) val b: Int = 0)

// Not a Moshi class: the checks only run on @JsonClass(generateAdapter = true) types.
data class Plain(@Json(name = "") val a: Int)
