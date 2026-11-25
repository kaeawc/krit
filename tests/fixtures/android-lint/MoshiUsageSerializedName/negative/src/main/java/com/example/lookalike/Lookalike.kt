package com.example.lookalike

import com.squareup.moshi.JsonClass

annotation class SerializedName(val value: String)

@JsonClass(generateAdapter = true)
data class Lookalike(@SerializedName("x") val x: String)
