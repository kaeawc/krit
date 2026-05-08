package test;

class Crypto {
    void hash() throws Exception {
        java.security.MessageDigest.getInstance("SHA-256");
        java.security.MessageDigest.getInstance("SHA-384");
        java.security.MessageDigest.getInstance("SHA-512");
        java.security.MessageDigest.getInstance("SHA3-256");
        MessageDigest.getInstance("MD5");
    }
}

class MessageDigest {
    static MessageDigest getInstance(String name) {
        return new MessageDigest();
    }
}
