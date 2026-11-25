package com.example

import com.squareup.moshi.JsonClass
import dev.zacsweers.moshix.sealed.annotations.TypeLabel

@JsonClass(generateAdapter = true, generator = "sealed:type")
sealed class Result {
  @TypeLabel("success")
  @JsonClass(generateAdapter = true)
  data class Success<T : Any>(val value: T) : Result()
}
