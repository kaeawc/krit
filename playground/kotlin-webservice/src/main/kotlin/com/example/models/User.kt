package com.example.models

import kotlinx.serialization.Serializable

@Serializable
data class User(
    val id: String,
    val name: String,
    val email: String,
    val age: Int,
    val role: String = "user"
)
