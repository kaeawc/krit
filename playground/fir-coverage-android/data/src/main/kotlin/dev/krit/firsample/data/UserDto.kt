package dev.krit.firsample.data

import com.squareup.moshi.JsonClass

// Moshi's KSP processor generates UserDtoJsonAdapter for this class.
@JsonClass(generateAdapter = true)
data class UserDto(
    val id: String,
    val name: String,
    val visits: Int = 0,
)
