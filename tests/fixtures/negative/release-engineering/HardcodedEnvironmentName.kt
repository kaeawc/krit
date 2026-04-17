package com.example.release

object BuildConfig {
    const val ENV = "staging"
}

fun apiConfig(environment: String): String = environment

fun createApi(): String {
    return apiConfig(BuildConfig.ENV)
}
