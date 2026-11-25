package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
interface AnInterface

@JsonClass(generateAdapter = true)
abstract class AnAbstract(val a: Int)

@JsonClass(generateAdapter = true)
annotation class AnAnnotation

@JsonClass(generateAdapter = true)
sealed class ASealed

class Outer {
  @JsonClass(generateAdapter = true)
  inner class Inner(val a: Int)
}
