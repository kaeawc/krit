package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class WithPrimary(val a: Int)

@JsonClass(generateAdapter = true)
class EmptyPrimary()

class Holder {
  companion object {
    @JsonClass(generateAdapter = true)
    data class InCompanion(val a: Int)
  }
}
