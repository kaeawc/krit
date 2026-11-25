package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true, generator = "sealed:type")
abstract class AbstractBase

@JsonClass(generateAdapter = true, generator = "sealed:type")
interface PlainInterface
