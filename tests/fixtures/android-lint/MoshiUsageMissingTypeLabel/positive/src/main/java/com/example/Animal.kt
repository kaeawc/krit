package com.example

import com.squareup.moshi.JsonClass
import dev.zacsweers.moshix.sealed.annotations.TypeLabel

@JsonClass(generateAdapter = true, generator = "sealed:type")
sealed class Animal {
  @TypeLabel("dog")
  @JsonClass(generateAdapter = true)
  data class Dog(val name: String) : Animal()

  @JsonClass(generateAdapter = true)
  data class Cat(val name: String) : Animal()
}

class Zoo {
  @JsonClass(generateAdapter = true, generator = "sealed:kind")
  sealed interface Exhibit {
    data class Tank(val litres: Int) : Exhibit
  }
}
