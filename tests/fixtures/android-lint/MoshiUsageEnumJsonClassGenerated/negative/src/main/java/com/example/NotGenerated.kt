package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = false)
enum class NotGenerated { UNKNOWN, ONE }

// A custom generator owns the enum, so generateAdapter = true is fine.
@JsonClass(generateAdapter = true, generator = "custom")
enum class CustomGenerator { UNKNOWN, ONE }

@Suppress("MoshiUsageEnumJsonClassGenerated")
@JsonClass(generateAdapter = true)
enum class Suppressed { UNKNOWN, ONE }
