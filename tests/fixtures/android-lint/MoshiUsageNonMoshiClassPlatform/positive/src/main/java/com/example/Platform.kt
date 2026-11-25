package com.example

import com.squareup.moshi.JsonClass
import java.time.Instant
import java.util.Date
import java.util.UUID

@JsonClass(generateAdapter = true)
data class Platform(
  val date: Date,
  val uuid: UUID,
  val instant: Instant,
  val pair: Pair<String, Int>,
  val callback: (String) -> Unit,
  // Instant is final, so Kotlin passes it to List without a wildcard and Lint checks it.
  val instants: List<Instant>,
  val wrapped: Wrapper<Date>,
)

@JsonClass(generateAdapter = true)
data class Wrapper<T>(val value: T)
