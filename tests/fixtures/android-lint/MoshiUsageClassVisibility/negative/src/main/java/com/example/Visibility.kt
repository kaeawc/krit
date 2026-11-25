package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class PublicModel(val a: Int)

@JsonClass(generateAdapter = true)
public data class ExplicitPublicModel(val a: Int)

@JsonClass(generateAdapter = true)
internal data class InternalModel(val a: Int)

// Not a Moshi class.
private data class PrivatePlain(val a: Int)
