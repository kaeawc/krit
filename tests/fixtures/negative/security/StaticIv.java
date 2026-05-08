package test;

import java.security.SecureRandom;

class Crypto {
    private byte[] field = new byte[16];
    void params(byte[] param) {
        byte[] random = new byte[16];
        new SecureRandom().nextBytes(random);
        new javax.crypto.spec.IvParameterSpec(param);
        new javax.crypto.spec.IvParameterSpec(field);
        new javax.crypto.spec.IvParameterSpec(random);
    }
}
