package com.example.lookalike

import com.squareup.moshi.JsonClass

annotation class TypeLabel(val label: String)

// A local TypeLabel is not moshix's.
@TypeLabel("local")
@JsonClass(generateAdapter = true)
data class Lookalike(val x: Int)
