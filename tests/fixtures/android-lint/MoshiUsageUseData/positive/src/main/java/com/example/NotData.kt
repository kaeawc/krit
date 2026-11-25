package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
class NotData(val a: Int)

@JsonClass(generateAdapter = true)
open class OpenModel(val a: Int)
