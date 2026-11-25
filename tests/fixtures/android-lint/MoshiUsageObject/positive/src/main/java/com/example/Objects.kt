package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
object Singleton

class Holder {
  @JsonClass(generateAdapter = true)
  companion object Factory
}
