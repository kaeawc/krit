package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class NoSites(@Json(name = "a") val a: Int)

// Site targets on other annotations are fine.
@JsonClass(generateAdapter = true)
data class OtherSites(@field:Deprecated("x") val a: Int)
