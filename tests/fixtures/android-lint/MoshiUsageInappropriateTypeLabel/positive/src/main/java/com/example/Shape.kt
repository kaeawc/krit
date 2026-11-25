package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true, generator = "sealed:kind")
sealed interface Shape
