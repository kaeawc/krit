package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = false)
enum class Status { UNKNOWN, ONE }

enum class Plain { UNKNOWN, ONE }

@JsonClass(generateAdapter = true)
data class Holder(
  val status: Status = Status.ONE,
  val nullable: Status? = null,
  // Not a Moshi enum: reported as EnumPropertyCouldBeMoshi instead.
  @Suppress("MoshiUsageEnumPropertyCouldBeMoshi") val plain: Plain = Plain.UNKNOWN,
)
