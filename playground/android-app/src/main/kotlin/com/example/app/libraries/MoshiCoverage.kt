package com.example.app.libraries

import com.squareup.moshi.JsonClass
import com.squareup.moshi.Moshi

class MoshiLibraryCoverage {
    private val moshi = Moshi.Builder().build()
    private val adapter = moshi.adapter(UserPayload::class.java)

    fun decodeUser(json: String): UserPayload? = adapter.fromJson(json)

    fun encodeUser(user: UserPayload): String = adapter.toJson(user)
}

@JsonClass(generateAdapter = false)
data class UserPayload(
    val id: String,
    val name: String,
)
