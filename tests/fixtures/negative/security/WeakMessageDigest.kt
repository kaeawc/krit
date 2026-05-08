package test

class Crypto {
    fun hash() {
        java.security.MessageDigest.getInstance("SHA-256")
        java.security.MessageDigest.getInstance("SHA-384")
        java.security.MessageDigest.getInstance("SHA-512")
        java.security.MessageDigest.getInstance("SHA3-256")
        MessageDigest.getInstance("MD5")
    }
}

class MessageDigest {
    companion object {
        fun getInstance(name: String): MessageDigest = MessageDigest()
    }
}
