package com.example.lookalike;

import com.squareup.moshi.JsonAdapter;

public enum JavaLookalike {
  UNKNOWN,
  @Json(name = "a") A;

  static Class<?> adapterType() {
    return JsonAdapter.class;
  }
}
