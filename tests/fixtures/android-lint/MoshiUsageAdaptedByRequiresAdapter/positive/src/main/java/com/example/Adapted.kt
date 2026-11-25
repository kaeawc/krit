package com.example

import androidx.annotation.Keep
import com.squareup.moshi.JsonClass
import dev.zacsweers.moshix.adapters.AdaptedBy

@AdaptedBy(NotAnAdapter::class)
data class Wrong(val a: Int)

@AdaptedBy(adapter = NotAnAdapter::class, nullSafe = false)
data class Named(val a: Int)

@JsonClass(generateAdapter = true)
data class Props(@AdaptedBy(NotAnAdapter::class) val p: Int)

@Keep
class NotAnAdapter
