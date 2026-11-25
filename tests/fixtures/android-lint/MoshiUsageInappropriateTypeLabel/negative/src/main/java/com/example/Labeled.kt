package com.example

import com.squareup.moshi.JsonClass
import dev.zacsweers.moshix.sealed.annotations.TypeLabel

@JsonClass(generateAdapter = true, generator = "sealed:kind")
sealed interface Shape

@TypeLabel("circle")
@JsonClass(generateAdapter = true)
data class Circle(val r: Int) : Shape
