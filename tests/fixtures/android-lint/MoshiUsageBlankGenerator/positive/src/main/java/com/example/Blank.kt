package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true, generator = "")
data class Empty(val a: Int)

@JsonClass(generateAdapter = true, generator = "  ")
data class Spaces(val a: Int)

@JsonClass(true, "")
data class Positional(val a: Int)
