package com.example

import com.squareup.moshi.Json

enum class Status {
  UNKNOWN,
  @Json(name = "active") ACTIVE,
}

class Holder {
  enum class Nested {
    UNKNOWN,
    @com.squareup.moshi.Json(name = "n") NESTED,
  }
}
