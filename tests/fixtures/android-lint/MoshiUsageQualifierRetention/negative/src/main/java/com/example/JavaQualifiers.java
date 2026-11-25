package com.example;

import static java.lang.annotation.RetentionPolicy.RUNTIME;

import com.squareup.moshi.JsonQualifier;
import java.lang.annotation.Retention;

public class JavaQualifiers {
  @JsonQualifier
  @Retention(RUNTIME)
  @interface Runtime {}

  // Not a JsonQualifier.
  @interface Plain {}
}
