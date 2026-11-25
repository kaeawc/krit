package com.example;

import com.squareup.moshi.JsonClass;

@JsonClass(generateAdapter = false)
public enum JavaHasUnknown {
  UNKNOWN,
  ONE
}
