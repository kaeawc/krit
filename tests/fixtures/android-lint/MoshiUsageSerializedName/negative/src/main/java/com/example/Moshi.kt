package com.example

import com.google.gson.annotations.SerializedName
import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Moshi(@Json(name = "full") val full: String)

// A Gson model is not a Moshi class.
data class GsonModel(@SerializedName("name") val name: String)
