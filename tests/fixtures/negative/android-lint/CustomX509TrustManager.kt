package com.example

class PlainTrust {
    fun accepts(host: String): Boolean = host.isNotEmpty()
}
