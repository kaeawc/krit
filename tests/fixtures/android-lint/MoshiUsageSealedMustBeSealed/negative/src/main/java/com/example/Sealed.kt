package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true, generator = "sealed:type")
sealed class SealedBase

@JsonClass(generateAdapter = true, generator = "sealed:type")
sealed interface SealedInterface

// Other generators may target non-sealed types.
@JsonClass(generateAdapter = true, generator = "custom")
abstract class CustomBase
