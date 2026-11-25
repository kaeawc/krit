package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class IsData(val a: Int)

// Not annotated.
class Plain(val a: Int)

@Suppress("MoshiUsageUseData")
@JsonClass(generateAdapter = true)
class Suppressed(val a: Int)
