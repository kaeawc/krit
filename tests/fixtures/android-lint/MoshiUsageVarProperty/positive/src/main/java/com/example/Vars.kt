package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Vars(var a: Int, val b: Int, var c: String = "")
