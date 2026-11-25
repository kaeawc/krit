package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = false)
enum class Casing { UNKNOWN, ONE, TWO_WORDS, V2 }

// Not a Moshi enum.
enum class Plain { lower, camelCase }

// A custom generator skips the casing check.
@JsonClass(generateAdapter = false, generator = "custom")
enum class Custom { UNKNOWN, lower }

@JsonClass(generateAdapter = false)
enum class Suppressed {
  UNKNOWN,
  @Suppress("MoshiUsageEnumCasing") lower,
}
