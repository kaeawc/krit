package com.example.exceptions

class ConfigLoader {

    fun loadConfig(path: String): String {
        if (path.isEmpty()) {
            throw Exception("bad path")
        }
        if (!path.endsWith(".yml")) {
            throw Throwable("unsupported format")
        }
        return readConfig(path)
    }

    private fun readConfig(path: String): String = "config"
}
