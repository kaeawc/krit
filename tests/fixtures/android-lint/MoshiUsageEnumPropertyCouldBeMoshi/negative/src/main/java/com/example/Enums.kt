package com.example

import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = false)
enum class Status { UNKNOWN, ONE }

@JsonClass(generateAdapter = true)
data class Holder(val status: Status, val statuses: List<Status>)

enum class Plain { UNKNOWN, ONE }

// Kotlin passes an enum type argument as a wildcard (List<? extends Plain>), which Lint does not check.
@JsonClass(generateAdapter = true)
data class ListHolder(val plains: List<Plain>)
