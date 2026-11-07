package com.example;

import java.security.SecureRandom;

class DefaultRandom {
    SecureRandom create() {
        return new SecureRandom();
    }
}
