package test;

import javax.crypto.Cipher;

class KeyCipher {
    String getInstance(String transformation) { return transformation; }
}

class Crypto {
    void cipher() throws Exception {
        javax.crypto.Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding");
        javax.crypto.Cipher.getInstance("RSA/ECB/PKCS1Padding");
        Cipher.getInstance("AES/GCM/NoPadding");
    }

    String parameter(KeyCipher Cipher) {
        return Cipher.getInstance("RSA/ECB/NoPadding");
    }

    String localVariable() {
        KeyCipher Cipher = new KeyCipher();
        return Cipher.getInstance("RSA/ECB/NoPadding");
    }
}
