package com.example;

import java.security.SecureRandom;

class JavaTokenGenerator {
    long literalSeed() {
        SecureRandom rng = new SecureRandom();
        rng.setSeed(1L);
        return rng.nextLong();
    }

    long timeSeed() {
        SecureRandom rng = new SecureRandom();
        rng.setSeed(System.nanoTime());
        return rng.nextLong();
    }
}
