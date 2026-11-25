package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
private data class PrivateModel(val a: Int)

class Outer {
  @JsonClass(generateAdapter = true)
  protected data class ProtectedModel(val a: Int)
}
