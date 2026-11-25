package com.example

import com.squareup.moshi.JsonAdapter
import dev.zacsweers.moshix.adapters.AdaptedBy

@AdaptedBy(NotKept::class)
data class NoKeep(val a: Int)

@AdaptedBy(JavaNotKept::class)
data class ViaJava(val a: Int)

abstract class NotKept : JsonAdapter<NoKeep>()
