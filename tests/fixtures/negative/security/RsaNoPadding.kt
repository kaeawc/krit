package test

class Crypto {
    fun cipher() {
        javax.crypto.Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding")
        javax.crypto.Cipher.getInstance("RSA/ECB/PKCS1Padding")
        javax.crypto.Cipher.getInstance("AES/GCM/NoPadding")
    }
}
