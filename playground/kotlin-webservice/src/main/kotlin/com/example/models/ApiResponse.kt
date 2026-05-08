package com.example.models

import kotlinx.serialization.Serializable

@Serializable
data class ApiResponse<T>(
    val data: T?,
    val success: Boolean,
    val message: String
)
