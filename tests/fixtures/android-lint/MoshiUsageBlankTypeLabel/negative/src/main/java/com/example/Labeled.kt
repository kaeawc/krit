package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true, generator = "sealed:type")
sealed class Labeled

// Not a moshi-sealed generator.
@JsonClass(generateAdapter = true, generator = "custom:")
class Custom
