package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
class Secondary {
  constructor(a: Int)
}

@JsonClass(generateAdapter = true)
class NoParens

// Lint does not see a value class's constructor as a primary constructor.
@JvmInline
@JsonClass(generateAdapter = true)
value class Wrapper(val a: Int)
