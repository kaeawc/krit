package com.example.exceptions

import java.io.IOException

class NetworkClient {

    fun fetchData(): String {
        try {
            return loadFromNetwork()
        } catch (e: IOException) {
            log(e)
            return "network error"
        }
    }

    private fun loadFromNetwork(): String = "data"

    private fun log(e: Exception) {
        println(e.message)
    }
}
