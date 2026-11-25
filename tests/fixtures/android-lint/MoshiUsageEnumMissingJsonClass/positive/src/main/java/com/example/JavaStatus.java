package com.example;

import com.squareup.moshi.Json;

public enum JavaStatus {
  UNKNOWN,
  @Json(name = "active") ACTIVE
}
