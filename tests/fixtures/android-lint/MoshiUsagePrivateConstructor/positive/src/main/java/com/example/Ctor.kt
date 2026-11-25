package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class PrivateCtor private constructor(val a: Int)

@JsonClass(generateAdapter = true)
data class ProtectedCtor protected constructor(val a: Int)
