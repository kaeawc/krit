package com.example;

import com.google.auto.value.AutoValue;

@AutoValue
abstract class Address {
  abstract String street();

  @AutoValue.Builder
  abstract static class Builder {
    abstract Address build();
  }
}

record Range(int start, int end) {}
