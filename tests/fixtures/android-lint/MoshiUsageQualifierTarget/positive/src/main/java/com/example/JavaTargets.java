package com.example;

import static java.lang.annotation.ElementType.METHOD;
import static java.lang.annotation.RetentionPolicy.RUNTIME;

import com.squareup.moshi.JsonQualifier;
import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.Target;

public class JavaTargets {
  @JsonQualifier
  @Retention(RUNTIME)
  @Target(METHOD)
  @interface MethodOnly {}

  @JsonQualifier
  @Retention(RUNTIME)
  @Target({ElementType.METHOD, ElementType.PARAMETER})
  @interface ArrayNoField {}
}
