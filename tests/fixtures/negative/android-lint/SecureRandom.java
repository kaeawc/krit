package com.example;

import java.security.SecureRandom;

class JavaTokenGenerator {
    long byteArraySeed(byte[] seedBytes) {
        SecureRandom rng = new SecureRandom();
        rng.setSeed(seedBytes);
        return rng.nextLong();
    }

    static class Seeder {
        void setSeed(long seed) {}
    }

    void unrelatedSetSeed() {
        Seeder rng = new Seeder();
        rng.setSeed(1L);
    }
}
