package com.example.util

// This class is referenced by 50+ files across the project (high fan-in hotspot)
object StringUtils {
    fun capitalize(s: String): String = s.replaceFirstChar { it.uppercase() }
    fun truncate(s: String, max: Int): String = if (s.length > max) s.take(max) + "..." else s
    fun isBlank(s: String): Boolean = s.trim().isEmpty()
    fun toCamelCase(s: String): String = s.split("_").joinToString("") { it.capitalize() }
    fun toSnakeCase(s: String): String = s.replace(Regex("[A-Z]")) { "_${it.value.lowercase()}" }
    fun padLeft(s: String, len: Int, c: Char = ' '): String = s.padStart(len, c)
    fun padRight(s: String, len: Int, c: Char = ' '): String = s.padEnd(len, c)
    fun removeWhitespace(s: String): String = s.replace(Regex("\\s"), "")
    fun reverse(s: String): String = s.reversed()
    fun countOccurrences(s: String, sub: String): Int = s.windowed(sub.length).count { it == sub }
}
