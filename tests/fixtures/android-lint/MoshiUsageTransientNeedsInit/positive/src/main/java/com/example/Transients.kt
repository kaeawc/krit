package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Transients(
  val a: Int,
  @Transient val cached: Int,
  @field:Transient val field: Int,
  @kotlin.jvm.Transient val qualified: Int,
)

@JsonClass(generateAdapter = true)
class NonProperty(val a: Int, @Transient plain: Int)
