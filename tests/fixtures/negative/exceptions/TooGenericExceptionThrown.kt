package com.example.exceptions

class ConfigLoader {

    fun loadConfig(path: String): String {
        if (path.isEmpty()) {
            throw IllegalArgumentException("path must not be empty")
        }
        if (!path.endsWith(".yml")) {
            throw IllegalStateException("unsupported config format: $path")
        }
        return readConfig(path)
    }

    private fun readConfig(path: String): String = "config"
}
