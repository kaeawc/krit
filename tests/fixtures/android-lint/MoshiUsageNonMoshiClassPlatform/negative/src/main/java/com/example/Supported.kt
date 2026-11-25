package com.example

import com.squareup.moshi.JsonClass
import java.util.Date

@JsonClass(generateAdapter = true)
data class Supported<T>(
  val name: String,
  val count: Int,
  val ratio: Double?,
  val value: T,
  val anything: Any,
  val names: List<String>,
  // Date is open, so List<Date> becomes List<? extends Date> and Lint does not check it.
  val dates: List<Date>,
  val star: List<*>,
  val projected: List<out Date>,
)
