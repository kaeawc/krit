package test

import javax.crypto.Cipher

/** A Cipher nested in another class does not shadow the import. */
class Holder {
    class Cipher
}

class Crypto {
    fun cipher() {
        Cipher.getInstance("RSA/ECB/NoPadding")
        Cipher.getInstance("RSA/NONE/NoPadding")
    }
}
