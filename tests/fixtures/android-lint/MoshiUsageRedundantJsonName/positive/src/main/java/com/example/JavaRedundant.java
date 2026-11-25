package com.example;

import com.squareup.moshi.Json;
import com.squareup.moshi.JsonClass;

@JsonClass(generateAdapter = false)
public enum JavaRedundant {
  UNKNOWN,
  @Json(name = "ONE") ONE
}
