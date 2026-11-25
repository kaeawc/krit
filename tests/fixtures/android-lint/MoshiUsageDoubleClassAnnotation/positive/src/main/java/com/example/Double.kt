package com.example

import androidx.annotation.Keep
import com.squareup.moshi.JsonAdapter
import com.squareup.moshi.JsonClass
import dev.zacsweers.moshix.adapters.AdaptedBy

@AdaptedBy(DoubleAdapter::class)
@JsonClass(generateAdapter = true)
data class Double(val a: Int)

@Keep
abstract class DoubleAdapter : JsonAdapter<Double>()
