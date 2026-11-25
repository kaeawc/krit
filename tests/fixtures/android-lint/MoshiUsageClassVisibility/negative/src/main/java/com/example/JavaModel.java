package com.example;

import com.squareup.moshi.JsonClass;

// The class checks run on Kotlin only, as in Lint.
public class JavaModel {
  @JsonClass(generateAdapter = true)
  private static class PrivateJavaModel {}
}
