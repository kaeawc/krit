package test

class Crypto {
    fun mac() {
        javax.crypto.Mac.getInstance("HmacSHA256")
        javax.crypto.Mac.getInstance("HmacSHA384")
        javax.crypto.Mac.getInstance("HmacSHA512")
        Mac.getInstance("HmacSHA1")
    }
}

class Mac {
    companion object {
        fun getInstance(name: String): Mac = Mac()
    }
}
