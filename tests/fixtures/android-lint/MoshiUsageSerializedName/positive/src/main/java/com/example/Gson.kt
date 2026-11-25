package com.example

import com.google.gson.annotations.SerializedName
import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Gson(
  @SerializedName("full_gson") val fullGson: String,
  @Json(name = "mixed") @SerializedName("mixed") val mixed: String,
  @param:SerializedName("param") val param: String,
)

@JsonClass(generateAdapter = false)
enum class GsonEnum { UNKNOWN, @SerializedName("one") ONE }
