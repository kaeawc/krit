package com.example

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = false)
enum class Status {
  UNKNOWN,
  @Json(name = "active") ACTIVE,
}

// Not a Moshi enum: no @JsonClass and no @Json member. "@Json" in a comment does not count.
enum class Plain { UNKNOWN, ACTIVE }

@Suppress("MoshiUsageEnumMissingJsonClass")
enum class Suppressed {
  UNKNOWN,
  @Json(name = "s") SUPPRESSED,
}
