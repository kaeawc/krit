package test;

import javax.crypto.Mac;

class Crypto {
    void mac() throws Exception {
        Mac.getInstance("HmacMD5");
        Mac.getInstance("HmacSHA1");
    }
}
