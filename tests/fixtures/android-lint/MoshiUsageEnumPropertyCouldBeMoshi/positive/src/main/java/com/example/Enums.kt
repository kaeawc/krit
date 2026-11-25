package com.example

import com.squareup.moshi.JsonClass

enum class Plain { UNKNOWN, ONE }

@JsonClass(generateAdapter = true)
data class Holder(val plain: Plain, val plainDefault: Plain = Plain.ONE)
