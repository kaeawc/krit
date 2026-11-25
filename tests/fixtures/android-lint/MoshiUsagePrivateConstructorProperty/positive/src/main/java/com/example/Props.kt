package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Props(
  private val a: Int,
  protected val b: Int,
)
