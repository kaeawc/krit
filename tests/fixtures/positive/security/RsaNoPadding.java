package test;

import javax.crypto.Cipher;

class Crypto {
    void cipher() throws Exception {
        Cipher.getInstance("RSA/ECB/NoPadding");
        Cipher.getInstance("RSA/NONE/NoPadding");
    }
}
