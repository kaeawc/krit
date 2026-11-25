package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Transients(val a: Int, @Transient val cached: Int = 0)
