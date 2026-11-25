package com.example;

import com.squareup.moshi.Json;
import com.squareup.moshi.JsonClass;

@JsonClass(generateAdapter = false)
public enum JavaBlank {
  UNKNOWN,
  @Json(name = "") ONE
}
