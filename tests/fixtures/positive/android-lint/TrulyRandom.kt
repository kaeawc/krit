package test

import javax.crypto.KeyGenerator

fun createKey(): Any {
    return KeyGenerator.getInstance("AES")
}
