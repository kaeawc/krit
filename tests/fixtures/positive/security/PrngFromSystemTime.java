package test;

import java.util.Random;
import javax.crypto.Cipher;

class Crypto {
    void rng() {
        new Random(System.currentTimeMillis());
    }
}
