package com.example

import androidx.annotation.Keep
import com.squareup.moshi.JsonAdapter
import dev.zacsweers.moshix.adapters.AdaptedBy

@AdaptedBy(Direct::class)
data class ViaDirect(val a: Int)

@AdaptedBy(Factory::class)
data class ViaFactory(val a: Int)

@AdaptedBy(Indirect::class)
data class ViaBase(val a: Int)

@AdaptedBy(JavaAdapter::class)
data class ViaJava(val a: Int)

// An adapter from a library cannot be checked from source.
@AdaptedBy(okio.Buffer::class)
data class ViaLibrary(val a: Int)

@Keep abstract class Direct : JsonAdapter<ViaDirect>()

@Keep class Factory : JsonAdapter.Factory

abstract class Base<T> : JsonAdapter<T>()

@Keep abstract class Indirect : Base<ViaBase>()
