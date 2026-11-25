package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
class Params(val a: Int, nonProperty: String)
