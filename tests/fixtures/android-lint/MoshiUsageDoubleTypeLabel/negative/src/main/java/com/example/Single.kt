package com.example

import com.squareup.moshi.JsonClass
import dev.zacsweers.moshix.sealed.annotations.DefaultObject
import dev.zacsweers.moshix.sealed.annotations.TypeLabel

@JsonClass(generateAdapter = true, generator = "sealed:type")
sealed class Message {
  @TypeLabel("text")
  @JsonClass(generateAdapter = true)
  data class Text(val body: String) : Message()

  @DefaultObject
  object Unknown : Message()
}
