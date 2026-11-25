package com.example.lookalike

import com.squareup.moshi.JsonAdapter

// A local annotation that happens to be called Json is not Moshi's.
annotation class Json(val name: String)

enum class Lookalike {
  UNKNOWN,
  @Json(name = "a") A,
}

fun adapterType(): Class<*> = JsonAdapter::class.java
