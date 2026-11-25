package com.example

import com.squareup.moshi.JsonClass
import dev.zacsweers.moshix.sealed.annotations.DefaultObject
import dev.zacsweers.moshix.sealed.annotations.TypeLabel

@JsonClass(generateAdapter = true, generator = "sealed:type")
sealed class Animal {
  @TypeLabel("dog")
  @JsonClass(generateAdapter = true)
  data class Dog(val name: String) : Animal()

  @DefaultObject
  object Unknown : Animal()
}

// A sealed type without a moshi-sealed generator.
sealed class Plain {
  data class Leaf(val x: Int) : Plain()
}
