package com.example

import javax.crypto.Cipher

class Crypto {
    fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
}
