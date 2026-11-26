package com.example

import com.google.testing.junit.testparameterinjector.TestParameter

class TP(
  @TestParameter val a: String,
  @param:TestParameter val b: String,
  @field:TestParameter val c: String,
  @TestParameter d: String,
  @property:TestParameter val e: String,
) {
  constructor(@TestParameter val2: Int) : this("", "", "", "", "")
}
