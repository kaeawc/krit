package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class JavaHolder(val value: JavaPlain)
