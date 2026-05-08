package test;

class Crypto {
    void mac() throws Exception {
        javax.crypto.Mac.getInstance("HmacSHA256");
        javax.crypto.Mac.getInstance("HmacSHA384");
        javax.crypto.Mac.getInstance("HmacSHA512");
        Mac.getInstance("HmacSHA1");
    }
}

class Mac {
    static Mac getInstance(String name) {
        return new Mac();
    }
}
