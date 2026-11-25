package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class Interfaces(val list: List<String>, val set: Set<String>, val collection: Collection<String>)

// Not a Moshi class.
data class Plain(val list: ArrayList<String>)
