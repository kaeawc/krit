package test;

import java.security.SecureRandom;
import java.util.Random;

class Crypto {
    void rng(long seed) {
        new SecureRandom();
        new Random(seed);
    }
}
