package com.example.lookalike

import com.squareup.moshi.JsonClass

annotation class AdaptedBy(val adapter: kotlin.reflect.KClass<*>)

// A local AdaptedBy is not moshix's.
@AdaptedBy(String::class)
@JsonClass(generateAdapter = true)
data class Lookalike(val a: Int)
