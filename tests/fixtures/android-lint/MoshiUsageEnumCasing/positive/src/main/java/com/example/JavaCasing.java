package com.example;

import com.squareup.moshi.JsonClass;

@JsonClass(generateAdapter = false)
public enum JavaCasing {
  UNKNOWN,
  mixedCase
}
