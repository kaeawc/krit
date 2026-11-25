package com.example;

import com.squareup.moshi.JsonQualifier;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;

public class JavaQualifiers {
  @JsonQualifier
  @interface NoRetention {}

  @JsonQualifier
  @Retention(RetentionPolicy.CLASS)
  @interface ClassRetention {}
}
