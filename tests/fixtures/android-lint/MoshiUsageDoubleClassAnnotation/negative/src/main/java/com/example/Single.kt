package com.example

import androidx.annotation.Keep
import com.squareup.moshi.JsonAdapter
import com.squareup.moshi.JsonClass
import dev.zacsweers.moshix.adapters.AdaptedBy

@AdaptedBy(SingleAdapter::class)
data class Adapted(val a: Int)

@JsonClass(generateAdapter = true)
data class Generated(val a: Int)

@Keep
abstract class SingleAdapter : JsonAdapter<Adapted>()
