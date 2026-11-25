package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = false)
data class NotGenerated(val a: Int)

@JsonClass(false)
data class Positional(val a: Int)
