package com.example

import com.google.ai.client.generativeai.GenerativeModel

private const val GEMINI_KEY = "AIzaSyA0000000000000000000000000000000"

class Assistant {
    val direct = GenerativeModel(modelName = "gemini-pro", apiKey = "AIzaSyB0000000000000000000000000000000")
    val viaConstant = GenerativeModel("gemini-pro", GEMINI_KEY)

    fun local(): GenerativeModel {
        val key = "AIzaSyC0000000000000000000000000000000"
        return GenerativeModel("gemini-pro", key)
    }
}
