package test;

import java.security.MessageDigest;

class Crypto {
    void hash() throws Exception {
        MessageDigest.getInstance("MD5");
        MessageDigest.getInstance("SHA1");
    }
}
