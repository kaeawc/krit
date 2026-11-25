package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Params(val a: Int) {
  constructor(a: Int, extra: String) : this(a)
}

@JsonClass(generateAdapter = true)
class Defaulted(val a: Int, nonProperty: String = "")
