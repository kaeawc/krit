package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class InternalCtor internal constructor(val a: Int)

@JsonClass(generateAdapter = true)
data class PublicCtor constructor(val a: Int)

// Not a Moshi class.
data class Plain private constructor(val a: Int)
