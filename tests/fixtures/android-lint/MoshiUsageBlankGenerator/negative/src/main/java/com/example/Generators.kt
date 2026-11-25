package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Default(val a: Int)

@JsonClass(generateAdapter = true, generator = "custom")
class Custom
