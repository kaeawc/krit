package test;

import java.security.KeyPairGenerator;
import javax.crypto.KeyGenerator;

class Crypto {
    void keys() throws Exception {
        KeyPairGenerator rsa = KeyPairGenerator.getInstance("RSA");
        rsa.initialize(1024);
        KeyGenerator aes = KeyGenerator.getInstance("AES");
        aes.init(64);
    }
}
