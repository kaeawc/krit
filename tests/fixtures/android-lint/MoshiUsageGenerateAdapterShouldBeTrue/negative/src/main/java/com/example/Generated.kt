package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Generated(val a: Int)

// generateAdapter = false on a plain class may be there for R8.
@JsonClass(generateAdapter = false)
class KeptForR8(val a: Int)
