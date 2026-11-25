package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = false)
enum class HasUnknown { UNKNOWN, ONE }

// A custom generator skips the UNKNOWN requirement.
@JsonClass(generateAdapter = false, generator = "custom")
enum class Custom { ONE }

// Not a Moshi enum.
enum class Plain { ONE, TWO }

@Suppress("MoshiUsageEnumMissingUnknown")
@JsonClass(generateAdapter = false)
enum class Suppressed { ONE }
