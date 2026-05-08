package test;

class Crypto {
    void keys(int size, Generator other) throws Exception {
        java.security.KeyPairGenerator rsa = java.security.KeyPairGenerator.getInstance("RSA");
        rsa.initialize(2048);
        javax.crypto.KeyGenerator aes = javax.crypto.KeyGenerator.getInstance("AES");
        aes.init(128);
        aes.init(size);
        other.initialize(1024);
    }
}

class Generator {
    void initialize(int size) {}
}
