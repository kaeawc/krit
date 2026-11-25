package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true, generator = "sealed:")
sealed class NoLabel

@JsonClass(generateAdapter = true, generator = "sealed:  ")
sealed interface SpacesLabel
