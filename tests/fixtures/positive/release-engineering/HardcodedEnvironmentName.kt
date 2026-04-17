package com.example.release

fun apiConfig(environment: String): String = environment

fun createApi(): String {
    return apiConfig("staging")
}
