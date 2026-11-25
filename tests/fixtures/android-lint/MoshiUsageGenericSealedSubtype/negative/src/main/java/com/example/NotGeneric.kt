package com.example

import com.squareup.moshi.JsonClass
import dev.zacsweers.moshix.sealed.annotations.TypeLabel

@JsonClass(generateAdapter = true, generator = "sealed:type")
sealed class Result {
  @TypeLabel("success")
  @JsonClass(generateAdapter = true)
  data class Success(val value: String) : Result()
}

// Generic, but not type-labeled.
class Box<T>(val value: T)
