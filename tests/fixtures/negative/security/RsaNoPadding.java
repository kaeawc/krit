package test;

class Crypto {
    void cipher() throws Exception {
        javax.crypto.Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding");
        javax.crypto.Cipher.getInstance("RSA/ECB/PKCS1Padding");
        javax.crypto.Cipher.getInstance("AES/GCM/NoPadding");
    }
}
