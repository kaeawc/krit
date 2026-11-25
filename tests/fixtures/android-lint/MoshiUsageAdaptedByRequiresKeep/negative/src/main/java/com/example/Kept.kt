package com.example

import androidx.annotation.Keep
import com.squareup.moshi.JsonAdapter
import dev.zacsweers.moshix.adapters.AdaptedBy

@AdaptedBy(Kept::class)
data class WithKeep(val a: Int)

@AdaptedBy(Nested.Adapter::class)
data class ViaNested(val a: Int)

@Keep abstract class Kept : JsonAdapter<WithKeep>()

class Nested {
  @Keep abstract class Adapter : JsonAdapter<ViaNested>()
}
