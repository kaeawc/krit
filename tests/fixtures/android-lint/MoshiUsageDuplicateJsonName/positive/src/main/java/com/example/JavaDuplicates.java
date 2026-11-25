package com.example;

import com.squareup.moshi.Json;
import com.squareup.moshi.JsonClass;

@JsonClass(generateAdapter = false)
public enum JavaDuplicates {
  UNKNOWN,
  @Json(name = "d") A,
  @Json(name = "d") B
}
