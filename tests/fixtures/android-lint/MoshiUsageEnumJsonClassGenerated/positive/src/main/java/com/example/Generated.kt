package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
enum class Generated { UNKNOWN, ONE }

@JsonClass(true)
enum class Positional { UNKNOWN, ONE }

@com.squareup.moshi.JsonClass(generator = "", generateAdapter = true)
enum class Qualified { UNKNOWN, ONE }
