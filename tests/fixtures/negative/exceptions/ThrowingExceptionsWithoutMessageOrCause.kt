package com.example.exceptions

class Validator {

    fun validate(input: String) {
        if (input.isEmpty()) {
            throw IllegalArgumentException("input must not be empty")
        }
        if (input.length > 100) {
            throw IllegalStateException("input exceeds maximum length of 100")
        }
    }
}
