package com.example;

import com.google.gson.annotations.SerializedName;
import com.squareup.moshi.JsonClass;

@JsonClass(generateAdapter = false)
public enum JavaGson {
  UNKNOWN,
  @SerializedName("one") ONE
}
