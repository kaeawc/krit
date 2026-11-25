package com.example

import com.google.ai.client.generativeai.GenerativeModel

class Assistant(private val key: String) {
    val fromConfig = GenerativeModel("gemini-pro", key)
    val fromBuildConfig = GenerativeModel("gemini-pro", BuildConfig.GEMINI_API_KEY)
    val tooShort = GenerativeModel("gemini-pro", "AIzaShort")
    val modelNameOnly = GenerativeModel("AIzaSyE0000000000000000000000000000000", key)
    val concatenated = GenerativeModel("gemini-pro", "AIza" + "SyF0000000000000000000000000000000")
}

class Lookalike {
    class GenerativeModel(val name: String, val apiKey: String)

    val local = GenerativeModel("gemini-pro", "AIzaSyG0000000000000000000000000000000")
}
