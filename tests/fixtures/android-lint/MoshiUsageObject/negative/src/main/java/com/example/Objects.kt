package com.example

import com.squareup.moshi.JsonClass

// Not annotated.
object Plain

// generateAdapter = false returns before the type checks.
@JsonClass(generateAdapter = false)
object KeptForR8

@JsonClass(generateAdapter = true, generator = "custom")
object CustomGenerated
