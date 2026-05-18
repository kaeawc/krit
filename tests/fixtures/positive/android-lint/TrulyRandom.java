package com.example;

import java.security.SecureRandom;

class SeededRandom {
    SecureRandom create() {
        return new SecureRandom(new byte[]{1, 2, 3});
    }
}
