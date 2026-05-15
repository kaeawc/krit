package dev.krit.intellij

import com.google.gson.Gson
import com.google.gson.JsonSyntaxException

object KritJsonParser {
    private val gson = Gson()

    fun parse(json: String): KritReport {
        if (json.isBlank()) {
            return KritReport()
        }
        return try {
            gson.fromJson(json, KritReport::class.java) ?: KritReport()
        } catch (_: JsonSyntaxException) {
            KritReport()
        }
    }
}

