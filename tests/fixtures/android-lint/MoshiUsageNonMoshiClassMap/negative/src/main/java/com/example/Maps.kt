package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Maps(val map: Map<String, Int>)
