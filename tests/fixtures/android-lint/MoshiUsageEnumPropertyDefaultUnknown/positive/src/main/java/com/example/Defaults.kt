package com.example

import com.squareup.moshi.JsonClass
import com.example.Status.UNKNOWN

@JsonClass(generateAdapter = false)
enum class Status { UNKNOWN, ONE }

@JsonClass(generateAdapter = true)
data class Holder(val status: Status = Status.UNKNOWN, val imported: Status = UNKNOWN)
