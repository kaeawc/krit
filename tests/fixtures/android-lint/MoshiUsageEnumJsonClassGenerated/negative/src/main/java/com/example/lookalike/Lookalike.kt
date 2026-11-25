package com.example.lookalike

import com.squareup.moshi.Json

annotation class JsonClass(val generateAdapter: Boolean)

@JsonClass(generateAdapter = true)
enum class Lookalike { UNKNOWN, @Json(name = "one") ONE }
