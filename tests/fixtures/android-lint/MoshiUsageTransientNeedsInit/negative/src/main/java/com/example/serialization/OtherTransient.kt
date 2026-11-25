package com.example.serialization

import com.squareup.moshi.JsonClass
import kotlinx.serialization.Transient

// kotlinx.serialization's Transient is not kotlin.jvm.Transient.
@JsonClass(generateAdapter = true)
data class OtherTransient(val a: Int, @Transient val skipped: Int)
